package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/farrasnazhif/ripple-be/internal/entity"
	"github.com/farrasnazhif/ripple-be/internal/repository"
)

var ErrFinalNotReady = errors.New("accept one completed output per shot and resolve affected shots before creating the final output")
var ErrVideoAssembly = errors.New("video assembly failed; check FFmpeg installation and source media availability")

type FinalOutputService struct {
	review   repository.ReviewRepository
	projects repository.ProjectRepository
	briefs   repository.BriefRepository
	shots    repository.StoryboardRepository
	jobs     repository.GenerationRepository
	storage  BriefStorage
}

func NewFinalOutputService(r repository.ReviewRepository, p repository.ProjectRepository, b repository.BriefRepository, s repository.StoryboardRepository, j repository.GenerationRepository, storage BriefStorage) *FinalOutputService {
	return &FinalOutputService{r, p, b, s, j, storage}
}
func (s *FinalOutputService) Create(ctx context.Context, user, project, kind string) (*entity.FinalOutput, error) {
	if kind != "image" && kind != "video" {
		return nil, ErrInvalidReview
	}
	p, err := s.projects.Get(ctx, user, project)
	if err != nil {
		return nil, err
	}
	b, err := s.briefs.Get(ctx, user, p.BriefID)
	if err != nil {
		return nil, err
	}
	state, err := s.review.Get(ctx, user, project)
	if err != nil {
		return nil, err
	}
	shots, err := s.shots.List(ctx, user, project)
	if err != nil {
		return nil, err
	}
	jobs, err := s.jobs.List(ctx, user, project)
	if err != nil {
		return nil, err
	}
	if len(shots) == 0 {
		return nil, ErrFinalNotReady
	}
	final := &entity.FinalOutput{BriefVersion: b.Version, Kind: kind, JobIDs: []string{}, Outputs: []string{}}
	sources := []string{}
	for _, shot := range shots {
		var accepted *entity.GenerationJob
		for i := range jobs {
			if jobs[i].ShotID == shot.ID && jobs[i].Kind == kind && jobs[i].Accepted && jobs[i].Status == "completed" && len(jobs[i].Outputs) > 0 {
				accepted = &jobs[i]
			}
		}
		if accepted == nil {
			return nil, ErrFinalNotReady
		}
		if unresolvedImpact(state, shot.ID, accepted.BriefVersion, accepted.ID) {
			return nil, ErrFinalNotReady
		}
		final.JobIDs = append(final.JobIDs, accepted.ID)
		sources = append(sources, accepted.Outputs[0])
	}
	for _, saved := range state.Finals {
		if saved.Kind == kind && saved.BriefVersion == b.Version && strings.Join(saved.JobIDs, ",") == strings.Join(final.JobIDs, ",") {
			if saved.ObjectKey != "" && s.storage != nil {
				saved.DownloadURL, err = s.storage.PresignDownload(ctx, saved.ObjectKey)
			}
			return &saved, err
		}
	}
	if kind == "image" {
		final.Outputs = sources
	} else {
		if s.storage == nil {
			return nil, ErrStorageUnavailable
		}
		ctx, cancel := context.WithTimeout(ctx, 4*time.Minute)
		defer cancel()
		path, cleanup, err := AssembleVideo(ctx, sources)
		if err != nil {
			return nil, err
		}
		defer cleanup()
		random := make([]byte, 16)
		if _, err = rand.Read(random); err != nil {
			return nil, err
		}
		final.ObjectKey = "finals/" + user + "/" + project + "/" + hex.EncodeToString(random) + ".mp4"
		upload, err := s.storage.PresignUpload(ctx, final.ObjectKey, "video/mp4")
		if err != nil {
			return nil, err
		}
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, "PUT", upload, f)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "video/mp4")
		req.ContentLength = info.Size()
		response, err := (&http.Client{Timeout: time.Minute}).Do(req)
		if err != nil {
			return nil, err
		}
		response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return nil, ErrStorageUnavailable
		}
		size, contentType, err := s.storage.Stat(ctx, final.ObjectKey)
		if err != nil || size != info.Size() || contentType != "video/mp4" {
			s.storage.Delete(context.WithoutCancel(ctx), final.ObjectKey)
			return nil, ErrStorageUnavailable
		}
	}
	if err = s.review.SaveFinal(ctx, user, project, final); err != nil {
		if final.ObjectKey != "" {
			s.storage.Delete(context.WithoutCancel(ctx), final.ObjectKey)
		}
		return nil, err
	}
	if final.ObjectKey != "" {
		final.DownloadURL, err = s.storage.PresignDownload(ctx, final.ObjectKey)
	}
	return final, err
}
func unresolvedImpact(state *entity.ProjectReview, shot string, based int, job string) bool {
	for _, change := range state.Changes {
		if change.BriefVersion <= based {
			continue
		}
		for _, impact := range change.Impacts {
			if impact.ShotID == shot && impact.Status == "needs_review" {
				resolved := false
				for _, d := range state.Decisions {
					if d.ShotID == shot && d.JobID != nil && *d.JobID == job && d.BriefVersion >= change.BriefVersion && (d.Action == "keep" || d.Action == "accept") {
						resolved = true
						break
					}
				}
				if !resolved {
					return true
				}
			}
		}
	}
	return false
}
func mediaHTTPClient() *http.Client {
	return &http.Client{Timeout: 45 * time.Second, Transport: &http.Transport{Proxy: nil, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			ip = ip.Unmap()
			if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || !ip.IsGlobalUnicast() || ip == netip.MustParseAddr("169.254.169.254") {
				return nil, errors.New("private media host rejected")
			}
		}
		if len(ips) == 0 {
			return nil, errors.New("no media host")
		}
		return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
	}}, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != "https" || len(via) > 5 {
			return errors.New("invalid media redirect")
		}
		return nil
	}}
}
func AssembleVideo(ctx context.Context, sources []string) (string, func(), error) {
	if len(sources) < 1 || len(sources) > 50 {
		return "", func() {}, ErrFinalNotReady
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return "", func() {}, ErrVideoAssembly
	}
	dir, err := os.MkdirTemp("", "ripple-final-")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() { os.RemoveAll(dir) }
	ok := false
	defer func() {
		if !ok {
			cleanup()
		}
	}()
	list := []string{}
	client := mediaHTTPClient()
	for i, source := range sources {
		input := filepath.Join(dir, fmt.Sprintf("source-%d.mp4", i))
		f, err := os.Create(input)
		if err != nil {
			return "", cleanup, err
		}
		var reader io.ReadCloser
		if strings.HasPrefix(source, "/demo-assets/") {
			if os.Getenv("ENV") == "production" || filepath.Base(source) != strings.TrimPrefix(source, "/demo-assets/") {
				f.Close()
				return "", cleanup, ErrVideoAssembly
			}
			assetDir := os.Getenv("DEMO_ASSET_DIR")
			if assetDir == "" {
				assetDir = "../ripple-fe/public/demo-assets"
			}
			reader, err = os.Open(filepath.Join(assetDir, filepath.Base(source)))
		} else {
			u, e := url.Parse(source)
			if e != nil || u.Scheme != "https" || u.User != nil {
				f.Close()
				return "", cleanup, ErrVideoAssembly
			}
			req, e := http.NewRequestWithContext(ctx, "GET", source, nil)
			if e != nil {
				f.Close()
				return "", cleanup, e
			}
			response, e := client.Do(req)
			err = e
			if e == nil {
				if response.StatusCode != 200 {
					response.Body.Close()
					err = ErrVideoAssembly
				} else {
					reader = response.Body
				}
			}
		}
		if err != nil {
			f.Close()
			return "", cleanup, ErrVideoAssembly
		}
		n, copyErr := io.Copy(f, io.LimitReader(reader, 250<<20+1))
		reader.Close()
		f.Close()
		if copyErr != nil || n > 250<<20 {
			return "", cleanup, ErrVideoAssembly
		}
		probe, err := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-f", "mov", "-show_entries", "stream=codec_type:format=duration", "-of", "json", input).Output()
		if err != nil {
			return "", cleanup, ErrVideoAssembly
		}
		var streams struct {
			Format struct {
				Duration string `json:"duration"`
			} `json:"format"`
			Streams []struct {
				CodecType string `json:"codec_type"`
			} `json:"streams"`
		}
		if json.Unmarshal(probe, &streams) != nil {
			return "", cleanup, ErrVideoAssembly
		}
		duration, parseErr := strconv.ParseFloat(streams.Format.Duration, 64)
		if parseErr != nil || duration <= 0 || duration > 300 {
			return "", cleanup, ErrVideoAssembly
		}
		audio := false
		for _, stream := range streams.Streams {
			audio = audio || stream.CodecType == "audio"
		}
		output := filepath.Join(dir, fmt.Sprintf("clip-%d.mp4", i))
		args := []string{"-v", "error", "-y", "-protocol_whitelist", "file,pipe", "-f", "mov", "-i", input}
		if !audio {
			args = append(args, "-f", "lavfi", "-i", "anullsrc=channel_layout=stereo:sample_rate=48000")
		}
		args = append(args, "-map", "0:v:0")
		if audio {
			args = append(args, "-map", "0:a:0")
		} else {
			args = append(args, "-map", "1:a:0")
		}
		args = append(args, "-vf", "scale=1280:720:force_original_aspect_ratio=decrease,pad=1280:720:(ow-iw)/2:(oh-ih)/2,setsar=1", "-r", "30", "-c:v", "libx264", "-preset", "veryfast", "-pix_fmt", "yuv420p", "-c:a", "aac", "-ar", "48000", "-ac", "2", "-t", strconv.FormatFloat(duration, 'f', 3, 64), "-shortest", output)
		if err = exec.CommandContext(ctx, "ffmpeg", args...).Run(); err != nil {
			return "", cleanup, ErrVideoAssembly
		}
		list = append(list, fmt.Sprintf("file 'clip-%d.mp4'", i))
	}
	if err = os.WriteFile(filepath.Join(dir, "clips.txt"), []byte(strings.Join(list, "\n")), 0600); err != nil {
		return "", cleanup, err
	}
	output := filepath.Join(dir, "final.mp4")
	if err = exec.CommandContext(ctx, "ffmpeg", "-v", "error", "-y", "-f", "concat", "-safe", "1", "-i", filepath.Join(dir, "clips.txt"), "-c", "copy", "-movflags", "+faststart", output).Run(); err != nil {
		return "", cleanup, ErrVideoAssembly
	}
	ok = true
	return output, cleanup, nil
}
