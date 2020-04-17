-- queries.sql
-- name: GetPatients :many
SELECT
  *
FROM patient;
-- name: GetPatient :one
SELECT
  *
FROM patient
WHERE
  id = $1
LIMIT
  1;
-- name: AddPatient :one
INSERT INTO patient (
    first_name, last_name, address, phone, email
  )
VALUES
  ($1, $2, $3, $4, $5) RETURNING *;