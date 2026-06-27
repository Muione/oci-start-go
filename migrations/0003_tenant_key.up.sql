-- Phase 3: store OCI tenant API private key encrypted-at-rest (AES-256-GCM,
-- base64 ciphertext incl. nonce+tag) in DB instead of a plaintext PEM file on
-- disk. Parity deviation from Java (which wrote a plaintext PEM file and kept
-- its path in tenant.key_file) — see plan D1. The legacy key_file column is
-- retained for import compatibility; new Go saves populate key_file_blob.

ALTER TABLE tenant ADD COLUMN key_file_blob TEXT;
