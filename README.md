# Cheapskate 🏠

**Finance tracking as it should be. Not as it has been.**

Cheapskate is a frugal, self-hosted finance tracker built for families who want to keep tabs on their spending without handing over their data to third-party aggregators. It's fast, simple, and built with a modern Go stack.

![Dashboard Preview](https://via.placeholder.com/800x400?text=Cheapskate+Dashboard+Preview)

## Features

- **⚡ Blazing Fast**: Server-side rendered HTML with Go & Templ.
- **📱 Responsive**: Mobile-first design using Tailwind CSS.
- **🔋 Batteries Included**: SQLite database included, no complex setup required.
- **🔒 Private**: Your data stays on your machine.
- **✨ Interactive**: Smooth transitions and SPA-feel using HTMX.

## The Stack

We believe in the power of simplicity and the "BORING" stack:

- **Language**: [Go](https://go.dev)
- **Templating**: [Templ](https://templ.guide)
- **Database**: [SQLite](https://sqlite.org) + [SQLC](https://sqlc.dev)
- **Frontend**: [HTMX](https://htmx.org) + [TailwindCSS](https://tailwindcss.com)

## Getting Started

You can spin up your own instance in seconds.

### Prerequisites

- **Go 1.25+**
- **Make**

### Quick Installation

1.  **Clone the repo**
    ```bash
    git clone https://github.com/calexandrepcjr/cheapskate-finance-tracker.git
    cd cheapskate-finance-tracker
    ```

2.  **Install Tooling**
    We use `sqlc` for type-safe SQL and `templ` for HTML generation.
    ```bash
    make tools
    ```

3.  **Run the Server**
    ```bash
    make run
    ```
    Visit `http://localhost:8080`. That's it!

## Contributing

Contributions are what make the open source community such an amazing place to learn, inspire, and create. Any contributions you make are **greatly appreciated**.

1.  Fork the Project
2.  Create your Feature Branch (`git checkout -b feature/AmazingFeature`)
3.  Commit your Changes (`git commit -m 'Add some AmazingFeature'`)
4.  Push to the Branch (`git push origin feature/AmazingFeature`)
5.  Open a Pull Request

## Development

Want to hack on Cheapskate? We've optimized the developer experience for you.

### Live Reloading

Start the development server with hot-reload enabled (requires `air`, installed via `make tools`):

```bash
make dev
```

The server will automatically rebuild and restart when you change any `.go`, `.templ`, or `.sql` file.

### Project Structure

```
├── android/              # Android wrapper (Kotlin + Gradle)
│   └── app/src/main/     # Manifest, layouts, Kotlin sources
├── client/
│   ├── assets/           # Static assets (images, css)
│   └── templates/        # Templ components (UI)
├── server/
│   ├── db/               # Database schema & generated queries
│   ├── handlers_*.go     # HTTP Handlers
│   └── main.go           # Entrypoint
└── Makefile              # Build recipes
```

## Android

Cheapskate runs natively on Android by embedding the Go server inside an APK. The Android app launches the server as a background process and displays the UI in a WebView — no cloud services required.

### How It Works

The Android wrapper (`android/` directory) is a thin Kotlin app that:

1. Extracts a statically-linked ARM64 Go binary from the APK assets on first launch
2. Starts the server as a subprocess (`localhost:8080`)
3. Loads the web UI in a full-screen WebView
4. Stops the server when the app goes to the background

Your data stays on your phone in a local SQLite database.

### Prerequisites

You need these tools installed on your **desktop/laptop** (the machine where you build):

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.25+ | Cross-compile the server binary |
| Make | any | Build automation |
| Java JDK | 17+ | Required by Gradle (21 recommended) |
| Android SDK | API 34 | Compile the Android app |

Check what you already have:

```bash
go version        # Go 1.25+
java -version     # Java 17+
make --version    # GNU Make
```

### Installing the Android SDK (Command-Line Only)

You do **not** need Android Studio. The command-line tools are enough (~1 GB).

> **Quick setup:** Run `make setup-android` to automate all the steps below.
> Use `go run ./scripts/android-setup check` to see what's already installed.

#### 1. Download Command-Line Tools

Go to [developer.android.com/studio#command-line-tools-only](https://developer.android.com/studio#command-line-tools-only) and download the **"Command line tools only"** package for your OS.

Extract it into a directory that will become your SDK root:

```bash
# Linux
mkdir -p ~/android-sdk/cmdline-tools
unzip commandlinetools-linux-*.zip -d ~/android-sdk/cmdline-tools
mv ~/android-sdk/cmdline-tools/cmdline-tools ~/android-sdk/cmdline-tools/latest

# macOS
mkdir -p ~/android-sdk/cmdline-tools
unzip commandlinetools-mac-*.zip -d ~/android-sdk/cmdline-tools
mv ~/android-sdk/cmdline-tools/cmdline-tools ~/android-sdk/cmdline-tools/latest
```

#### 2. Set Environment Variables

Add these to your `~/.bashrc`, `~/.zshrc`, or equivalent:

```bash
export ANDROID_HOME="$HOME/android-sdk"
export PATH="$ANDROID_HOME/cmdline-tools/latest/bin:$ANDROID_HOME/platform-tools:$ANDROID_HOME/emulator:$PATH"
```

Reload your shell (`source ~/.bashrc`) and verify:

```bash
sdkmanager --version
```

#### 3. Install Required SDK Packages

```bash
# Accept licenses first
sdkmanager --licenses

# Install the minimum required packages
sdkmanager "platform-tools" "platforms;android-34" "build-tools;34.0.0"
```

If you also want to run an **emulator**, install these additional packages:

```bash
# Emulator + a system image (pick ONE image line below)

sdkmanager "emulator"

# For x86_64 hosts (most desktops/laptops) — fast, hardware-accelerated:
sdkmanager "system-images;android-34;google_apis;x86_64"

# For Apple Silicon Macs (M1/M2/M3) — native ARM64:
sdkmanager "system-images;android-34;google_apis;arm64-v8a"
```

> **Note for emulator users:** Use Android API 30+ (Android 11+) system images.
> API 30+ includes ARM binary translation, which lets the ARM64 Go server binary
> run inside an x86_64 emulator. Older API levels will not work with the emulator.

#### Alternative: Android Studio

If you prefer a GUI, install [Android Studio](https://developer.android.com/studio). It bundles the SDK, emulator, and all tools. Open `android/` as a project and Studio handles everything automatically.

### Generating the Gradle Wrapper

The Gradle wrapper JAR is not checked into the repository (it's a binary). You need to generate it once before your first build.

**Option A — If you have Gradle installed** (e.g., `brew install gradle` or `apt install gradle`):

```bash
cd android
gradle wrapper --gradle-version=8.7
cd ..
```

**Option B — If you have Android Studio**, simply open the `android/` directory as a project. Studio generates the wrapper automatically.

After this step, `android/gradle/wrapper/gradle-wrapper.jar` should exist.

### Building the APK

Once the SDK and Gradle wrapper are set up, build with a single command:

```bash
make build-android-apk
```

This command does three things:
1. Cross-compiles the Go server for ARM64 (`bin/server-android-arm64`)
2. Copies the binary into `android/app/src/main/assets/server-arm64`
3. Runs Gradle to produce the debug APK

The output APK will be at:

```
android/app/build/outputs/apk/debug/app-debug.apk
```

### Testing on an Android Emulator

#### 1. Create a Virtual Device (AVD)

```bash
# Create an AVD named "cheapskate_test"
avdmanager create avd \
  --name "cheapskate_test" \
  --package "system-images;android-34;google_apis;x86_64" \
  --device "pixel_6"
```

On Apple Silicon Macs, replace `x86_64` with `arm64-v8a`.

#### 2. Start the Emulator

```bash
emulator -avd cheapskate_test
```

The emulator window will open showing the Android boot animation. Wait until you see the home screen (first boot takes 1-2 minutes).

> **Tip:** Add `-no-snapshot-save` if you want a clean state each time, or let it
> save snapshots for faster subsequent boots.

#### 3. Install and Launch

In a separate terminal:

```bash
# Verify the emulator is detected
adb devices
# Should show something like: emulator-5554  device

# Install the APK
adb install android/app/build/outputs/apk/debug/app-debug.apk

# Launch the app
adb shell am start -n com.cheapskate.app.debug/com.cheapskate.app.MainActivity
```

The app will show a loading screen for a few seconds while the Go server starts, then display the Cheapskate UI.

#### 4. Reinstall After Changes

After rebuilding with `make build-android-apk`:

```bash
adb install -r android/app/build/outputs/apk/debug/app-debug.apk
```

The `-r` flag reinstalls while keeping your local database intact.

### Testing on a Physical Android Device

#### 1. Enable Developer Options on Your Phone

1. Open **Settings > About phone**
2. Tap **Build number** 7 times (you'll see "You are now a developer!")
3. Go back to **Settings > System > Developer options**
4. Enable **USB debugging**

#### 2. Connect and Verify

Plug your phone into your computer via USB. You may see a prompt on the phone asking to "Allow USB debugging" — tap **Allow**.

```bash
adb devices
# Should show your device serial number, e.g.:
# ABCD1234  device
```

If you see `unauthorized` instead of `device`, check the prompt on your phone.

#### 3. Install the APK

```bash
adb install android/app/build/outputs/apk/debug/app-debug.apk
```

Find "Cheapskate" in your app drawer and tap to open.

#### 4. Wireless Debugging (Optional)

To debug without a USB cable (Android 11+):

1. Go to **Developer options > Wireless debugging** and enable it
2. Tap **Pair device with pairing code**
3. On your computer:

```bash
adb pair <phone-ip>:<pairing-port>    # Enter the pairing code when prompted
adb connect <phone-ip>:<debug-port>   # Connect for ongoing use
adb install android/app/build/outputs/apk/debug/app-debug.apk
```

### Debugging

#### Viewing Server Logs

The Go server output is streamed to Android's Logcat. To follow it in real time:

```bash
# All Cheapskate-related logs
adb logcat -s ServerManager:* MainActivity:*

# Raw server output (the Go process stdout/stderr)
adb logcat | grep "\[server\]"
```

#### Common Issues

| Problem | Cause | Fix |
|---------|-------|-----|
| App shows "Server failed to start" | Binary not in APK assets | Rebuild with `make build-android-apk` (not just `make build-android`) |
| Emulator: server crashes immediately | x86_64 image below API 30 | Use API 30+ image (has ARM binary translation) |
| `adb devices` shows nothing | USB debugging not enabled | Enable Developer Options + USB Debugging on device |
| `adb devices` shows `unauthorized` | USB prompt not accepted | Check phone screen for "Allow USB debugging?" dialog |
| Gradle build fails: "SDK not found" | `ANDROID_HOME` not set | Export `ANDROID_HOME` pointing to your SDK directory |
| `./gradlew: Permission denied` | Missing execute bit | Run `chmod +x android/gradlew` |
| `gradlew` fails: "no wrapper JAR" | Wrapper not generated | See [Generating the Gradle Wrapper](#generating-the-gradle-wrapper) |
| App stuck on loading screen | Server taking long on first run | Wait up to 15 seconds; check Logcat for errors |

#### Uninstalling

```bash
adb uninstall com.cheapskate.app.debug
```

### Remote Server Mode

The Android app can also connect to a Cheapskate server running on another machine (e.g., your desktop) instead of running the embedded server:

1. Open the app and tap the menu (three dots) > **Settings**
2. Toggle **"Use remote server"**
3. Enter your server's URL (e.g., `http://192.168.1.100:8080`)

This is useful for development — run `make dev` on your desktop and point the Android app at it.

## License

Cheapskate is open-source software licensed under the [MIT license](LICENSE).
