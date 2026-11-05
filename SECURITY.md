# Security and privacy notes

## Secrets

- Keep the Gnani API key only in `.env` or deployment environment variables.
- `.env` is excluded by `.gitignore` and `.dockerignore`.
- Do not put the key in `web/app.js`, screenshots, demo videos or repository commits.
- If a key is exposed, revoke it in the Gnani dashboard and create a replacement.

## Audio

The application keeps recorded audio in browser memory, sends it through the local Go backend for transcription and does not intentionally write the audio file to disk. Use synthetic voice commands and avoid personal identifiers in a public demo.

## Data

The included data is synthetic. Do not use real customer phone numbers, financial identifiers, Aadhaar, PAN or private conversations in the challenge demo.

## Prototype limits

This is a challenge prototype. A production deployment needs authentication, per-shop authorisation, encrypted database storage, audit retention, rate limiting, TLS, secret management and formal privacy review.
