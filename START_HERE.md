# Start here — Windows 10/11

This project needs only **Go** and a Chromium browser such as Chrome or Edge. There is no Node.js installation and no database setup.

## 1. Extract the ZIP

Extract the folder to:

```text
C:\Projects\vyapari-voice-copilot
```

Open that folder in VS Code.

## 2. Add your Gnani API key

In PowerShell, from the project folder:

```powershell
Copy-Item .env.example .env
notepad .env
```

Replace:

```text
GNANI_API_KEY=replace_with_your_gnani_api_key
```

with the key you created in the Gnani dashboard. Save the file. Never paste your key in screenshots, GitHub, LinkedIn or chat messages.

## 3. Run the application

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\run.ps1
```

Or run directly:

```powershell
go run .\cmd\server
```

Open:

```text
http://localhost:8080
```

## 4. Test in this order

1. Type `Basmati rice stock entha undi?` and run the command.
2. Click the microphone, allow browser permission and say the same sentence.
3. Say `Basmati rice rendu packets sale ayindi`.
4. Verify that stock decreases and the reply is spoken.
5. Say `Low stock products cheppu`.
6. Say `Ee roju sales entha?`.
7. Download the reorder CSV and inventory CSV.

## 5. Before recording the public demo

- Use only synthetic shop data.
- Make sure the green status pill shows the Gnani models as connected.
- Reset the demo from the top-right button.
- Close unrelated browser tabs and notifications.
- Record at 1080p if possible.
- Show the real microphone flow; do not present a fake or edited result.

## Troubleshooting

### Voice says API key is not configured

Check that the file is named exactly `.env`, not `.env.txt`, and restart the Go server.

### Microphone permission is blocked

Click the lock/microphone icon near the browser address bar, allow microphone access, then refresh.

### Port 8080 is already in use

Change this line in `.env`:

```text
APP_ADDR=:8081
```

Then open `http://localhost:8081`.

### Gnani returns insufficient credits or authentication error

Check **Usage** and **API Keys** in the Gnani dashboard. Confirm that the copied key has STT and TTS access.
