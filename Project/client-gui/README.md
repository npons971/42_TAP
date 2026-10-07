# README

## About

This is the official Wails Svelte template.

## Live Development

To run in live development mode, run `wails dev` in the project directory. This will run a Vite development
server that will provide very fast hot reload of your frontend changes. If you want to develop in a browser
and have access to your Go methods, there is also a dev server that runs on http://localhost:34115. Connect
to this in your browser, and you can call your Go code from devtools.

## Building

To build a redistributable, production mode package, use `wails build`.

## RFC protocol compatibility

The backend uses LOOK DETAILS, INVENTORY DETAILS and TALKJSON for descriptive
metadata; standard WHO, QUEST <npc> and QUESTS use RFC formats. Replies are
correlated with sent commands so empty quest lists do not clear inventory.
The default username is lowercase. Numeric errors and STATS/GROUP events are
handled. Backend networking tests can run independently of GTK:

```sh
GOPATH="$PWD/../../.go-work" GOCACHE="$PWD/../../.go-cache" ../../go test -race app.go app_test.go
cd frontend
../../../npm run build
```
