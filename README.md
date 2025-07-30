# Steam API Go Library

A Go library for integrating with the Steam API using purego for cross-platform dynamic library loading.

## Features

- Cross-platform support (Windows, Linux, macOS)
- Steam user authentication and ticket management
- DLC installation checks
- User information retrieval (Steam ID, username, language)
- Asynchronous callback handling
- Thread-safe operations

## Requirements

- Go 1.19+
- Steam client installed and running
- Steam API libraries for your platform

## Installation

```bash
go get github.com/slava-go-dev/go-steamworks-pure
```

## RUN

```bash
go build examples/main.go && ./main
```

## Steam API Libraries Setup

You need to place the appropriate Steam API libraries in a `libs` folder relative to your executable:

```
your-app/
├── your-app
├── steam_appid.txt
└── libs/
    ├── steam_api64.dll     (Windows)
    ├── libsteam_api64.so   (Linux)
    └── libsteam_api.dylib  (macOS)
```

Download the Steam API libraries from the [Steamworks SDK](https://partner.steamgames.com/doc/sdk#2).

## Platform-Specific Notes

### Windows
- Requires `steam_api64.dll` in the `libs` folder
- Supports both 32-bit and 64-bit applications

### Linux
- Requires `libsteam_api64.so` in the `libs` folder
- Ensure proper library permissions

### macOS
- Requires `libsteam_api.dylib` in the `libs` folder
- May require code signing for distribution

## Disclaimer

This library is not officially supported by Valve Corporation. Use at your own risk and ensure compliance with Steam's terms of service and API usage guidelines.