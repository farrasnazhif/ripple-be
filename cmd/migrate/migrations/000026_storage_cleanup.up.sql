CREATE TABLE storage_cleanup (
 object_key text PRIMARY KEY,
 -- Presigned uploads issued before deletion expire before cleanup is allowed.
 not_before timestamptz NOT NULL DEFAULT now() + interval '15 minutes'
);
CREATE INDEX storage_cleanup_due ON storage_cleanup(not_before);
