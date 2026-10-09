package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/farrasnazhif/ripple-be/internal/database"
	"github.com/farrasnazhif/ripple-be/internal/r2"
)

type demoShot struct {
	title, prompt string
	kinds         []string
}
type demoProject struct {
	key, name, description, brief, summary string
	shots                                  []demoShot
	failed                                 bool
}

var demos = []demoProject{
	{key: "draft", name: "DEMO 1 — Brief draft", description: "Start here: Raw Brief only. Review the uploaded reference, then generate or manually confirm a summary.", brief: "DEMO SOURCE: Launch a nature-inspired campaign for a new reusable water bottle. Audience: people who enjoy weekend walks. Tone: calm and optimistic. Deliverables: a landscape image and a five-second video. Preserve natural greens and warm morning light. This draft has no confirmed summary, storyboard or generated media."},
	{key: "images", name: "DEMO 2 — Image campaign", description: "Raw Brief → confirmed summary → storyboard → two sample image outputs. Open each node to understand the saved inputs and results.", brief: "DEMO SOURCE: Create two landscape campaign images for a nature-inspired product launch. Shot 1 establishes a peaceful outdoor setting. Shot 2 presents a bold graphic interpretation of the same palette. Use greens, warm sunlight and a clean composition. These demo outputs are locally drawn illustrations, not Higgsfield generations.", summary: "Two-image nature campaign with a calm outdoor establishing shot and a graphic companion image. Keep warm morning light, greens and clean composition. DEMO: sample illustrations stand in for generated outputs.", shots: []demoShot{{"Outdoor setting", "DEMO SAMPLE: A calm outdoor establishing composition with warm sunlight and layered green hills.", []string{"image"}}, {"Graphic companion", "DEMO SAMPLE: A graphic landscape composition in warm greens and gold, with open space for campaign copy.", []string{"image"}}}},
	{key: "mixed", name: "DEMO 3 — Mixed media and failed job", description: "Raw Brief → summary → storyboard → image and video branches. A failed video attempt is retained alongside its successful sample replacement.", brief: "DEMO SOURCE: Create an image and a short video for each of two nature-themed scenes. Scene 1 introduces a garden with warm light. Scene 2 focuses on flowers and gentle motion. Keep images and videos under their original storyboard shots. Demonstrate a failed video attempt followed by a successful attempt. DEMO: images are drawn locally and videos use the public-domain MDN flower sample; no provider was called.", summary: "Two nature-themed storyboard scenes, each with image and video deliverables. Retain failed generation history and successful replacements. DEMO: local illustrations and the MDN flower clip are sample media, not AI outputs.", shots: []demoShot{{"Garden introduction", "DEMO SAMPLE: Introduce a sunlit garden with a calm, natural composition. The local illustration and flower clip are placeholders.", []string{"image", "video"}}, {"Flower detail", "DEMO SAMPLE: Focus on flowers and gentle motion in warm light. The public-domain flower clip demonstrates the video output player.", []string{"image", "video"}}}, failed: true},
	{key: "summary", name: "DEMO 4 — Brief and summary only", description: "Two-node starting point: review the source brief and its confirmed summary, then choose images or video when you are ready.", brief: "DEMO SOURCE: Launch a reusable water bottle campaign for weekend hikers. Audience: adults who enjoy short outdoor adventures. Tone: calm, practical and optimistic. Use natural greens, warm morning light and a clean composition with space for campaign copy. Deliverables will be chosen after the brief summary is reviewed. Avoid unsupported product claims. This demo has no image plan, storyboard, generation jobs or final output.", summary: "DEMO CONFIRMED SUMMARY: A reusable bottle campaign for weekend hikers, using a calm and practical tone, natural greens, warm morning light, and open space for copy. Avoid unsupported product claims. Image or video output has not yet been selected; review this summary and confirm the requirements before continuing."},
	{key: "storyboard", name: "DEMO 5 — Storyboard ready", description: "Three-node flow: Brief, Brief Summary and Storyboard. Review the planned shots before generating any clips.", brief: "DEMO SOURCE: Create a calm, five-second nature campaign video for a reusable water bottle. Audience: weekend hikers. Plan an outdoor establishing shot, a close-up of the bottle beside greenery, and a closing landscape composition with open space for campaign copy. Preserve natural greens, warm morning light and a practical, optimistic tone. Avoid unsupported product claims. This demo stops at the reviewed storyboard; no media has been generated.", summary: "DEMO CONFIRMED SUMMARY: A nature-themed reusable bottle video for weekend hikers. The storyboard moves from a green outdoor setting to a bottle detail and a clean closing landscape. Maintain warm morning light, natural greens and room for copy. All shots are planned only; generation has not started.", shots: []demoShot{{title: "Outdoor introduction", prompt: "A calm outdoor establishing shot with layered green hills and warm morning light. Keep the composition uncluttered and the tone optimistic."}, {title: "Bottle detail", prompt: "A close-up of a reusable water bottle beside natural greenery in warm morning light. Avoid text, invented branding and unsupported product claims."}, {title: "Closing landscape", prompt: "A clean green landscape in warm morning light with open space for campaign copy. Preserve the calm, practical tone of the campaign."}}},
}

func main() {
	user := flag.String("user", "", "existing user UUID who will own the five demo projects")
	mediaDir := flag.String("media-dir", "../ripple-fe/public/demo-assets", "frontend public directory for sample media")
	flag.Parse()
	if !regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`).MatchString(*user) {
		log.Fatal("provide -user with an existing user UUID")
	}
	if _, err := os.Stat(filepath.Join(*mediaDir, "flower.mp4")); err != nil {
		log.Fatal("download the MDN flower sample to the media directory first; see README")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	db, err := database.New(database.Config{Addr: os.Getenv("DB_ADDR"), MaxOpenConns: 3, MaxIdleConns: 3, MaxIdleTime: "15m"})
	if err != nil {
		log.Fatal("database connection failed")
	}
	defer db.Close()
	storage, err := r2.New(os.Getenv("R2_ACCOUNT_ID"), os.Getenv("R2_ACCESS_KEY_ID"), os.Getenv("R2_SECRET_ACCESS_KEY"), os.Getenv("R2_BUCKET"))
	if err != nil {
		log.Fatal("configure R2 before seeding reference attachments")
	}
	if err = seed(ctx, db, storage, *user, *mediaDir); err != nil {
		log.Fatal(err)
	}
	if err = seedReviews(ctx, db, storage, *user); err != nil {
		log.Fatal(err)
	}
}

func seed(ctx context.Context, db *sql.DB, storage *r2.Client, user, mediaDir string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var active bool
	if err = tx.QueryRowContext(ctx, `SELECT is_active FROM users WHERE id=$1 FOR UPDATE`, user).Scan(&active); err != nil {
		return fmt.Errorf("demo owner not found: %w", err)
	}
	if !active {
		return fmt.Errorf("demo owner must be active")
	}
	uploaded := []string{}
	committed := false
	defer func() {
		if !committed {
			cleanup, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			for _, key := range uploaded {
				if err := storage.Delete(cleanup, key); err != nil {
					log.Printf("could not remove new demo reference %s", key)
				}
			}
		}
	}()
	for index, demo := range demos {
		projectID := demoID(user, demo.key)
		var exists bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM projects WHERE id=$1)`, projectID).Scan(&exists); err != nil {
			return err
		}
		if exists {
			fmt.Printf("Preserved existing %s (%s)\n", demo.name, projectID)
			continue
		}
		outputType := ""
		if demo.key == "storyboard" {
			outputType = "video"
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO projects(id,user_id,name,description,output_type)VALUES($1,$2,$3,$4,$5)`, projectID, user, demo.name, demo.description, outputType); err != nil {
			return err
		}
		var briefID string
		if err = tx.QueryRowContext(ctx, `INSERT INTO briefs(user_id,project_id,raw_text,summary)VALUES($1,$2,$3,NULLIF($4,''))RETURNING id`, user, projectID, demo.brief, demo.summary).Scan(&briefID); err != nil {
			return err
		}
		reference, err := sampleImage(index)
		if err != nil {
			return err
		}
		asset := fmt.Sprintf("landscape-%d.png", index+1)
		file, err := os.OpenFile(filepath.Join(mediaDir, asset), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err == nil {
			_, writeErr := file.Write(reference)
			closeErr := file.Close()
			if writeErr != nil {
				return writeErr
			}
			if closeErr != nil {
				return closeErr
			}
		} else if !os.IsExist(err) {
			return err
		}
		objectKey := "demo/" + user + "/" + projectID + "/" + briefID + "/reference.png"
		signed, err := storage.PresignUpload(ctx, objectKey, "image/png")
		if err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, signed, bytes.NewReader(reference))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "image/png")
		res, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
		if err != nil {
			return fmt.Errorf("demo reference upload failed")
		}
		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			return fmt.Errorf("demo reference upload HTTP %d", res.StatusCode)
		}
		uploaded = append(uploaded, objectKey)
		size, kind, err := storage.Stat(ctx, objectKey)
		if err != nil {
			return fmt.Errorf("could not verify demo reference")
		}
		if size != int64(len(reference)) || kind != "image/png" {
			return fmt.Errorf("demo reference verification mismatch")
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO brief_attachments(brief_id,object_key,file_name,content_type,size_bytes,status)VALUES($1,$2,$3,'image/png',$4,'completed')`, briefID, objectKey, "DEMO-reference.png", size); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO brief_versions(brief_id,version,raw_text,summary,attachments,actor_id) SELECT b.id,b.version,b.raw_text,COALESCE(b.summary,''),COALESCE((SELECT jsonb_agg(id) FROM brief_attachments WHERE brief_id=b.id AND status='completed'),'[]'),b.user_id FROM briefs b WHERE b.id=$1 ON CONFLICT(brief_id,version) DO NOTHING`, briefID); err != nil {
			return err
		}

		for position, shot := range demo.shots {
			var shotID string
			if err = tx.QueryRowContext(ctx, `INSERT INTO storyboard_shots(project_id,position,title,prompt)VALUES($1,$2,$3,$4)RETURNING id`, projectID, position+1, shot.title, shot.prompt).Scan(&shotID); err != nil {
				return err
			}
			if demo.failed && position == 0 {
				if err = insertJob(ctx, tx, shotID, "video", shot.prompt, "failed", nil); err != nil {
					return err
				}
			}
			for _, kind := range shot.kinds {
				output := "/demo-assets/" + asset
				if kind == "video" {
					output = "/demo-assets/flower.mp4"
				}
				if err = insertJob(ctx, tx, shotID, kind, shot.prompt, "completed", []string{output}); err != nil {
					return err
				}
			}
		}
		fmt.Printf("Prepared %s (%s)\n", demo.name, projectID)
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	committed = true
	fmt.Println("Demo projects saved. No ChatGPT or Higgsfield calls were made.")
	return nil
}
func insertJob(ctx context.Context, tx *sql.Tx, shotID, kind, prompt, status string, outputs []string) error {
	if outputs == nil {
		outputs = []string{}
	}
	payload, err := json.Marshal(outputs)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO generation_jobs(shot_id,request_key,kind,prompt,provider_id,status,outputs)VALUES($1,gen_random_uuid(),$2,$3,'demo-sample-no-provider-call',$4,$5)`, shotID, kind, prompt, status, string(payload))
	return err
}
func demoID(user, key string) string {
	hash := sha256.Sum256([]byte("ripple-demo:" + user + ":" + key))
	b := hash[:16]
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
func sampleImage(style int) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, 960, 540))
	palettes := [][3]color.RGBA{{{219, 239, 228, 255}, {85, 143, 112, 255}, {27, 88, 73, 255}}, {{250, 231, 178, 255}, {161, 184, 127, 255}, {64, 119, 88, 255}}, {{227, 218, 244, 255}, {149, 168, 195, 255}, {74, 118, 108, 255}}}
	colors := palettes[style%len(palettes)]
	for y := 0; y < 540; y++ {
		for x := 0; x < 960; x++ {
			c := colors[0]
			dx, dy := x-710, y-115
			if dx*dx+dy*dy < 55*55 {
				c = color.RGBA{255, 203, 91, 255}
			}
			if y > 280+(x%320)/4 {
				c = colors[1]
			}
			if y > 390-(x%400)/5 {
				c = colors[2]
			}
			img.SetRGBA(x, y, c)
		}
	}
	var buffer bytes.Buffer
	err := png.Encode(&buffer, img)
	return buffer.Bytes(), err
}
