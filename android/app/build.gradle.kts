/*
 * app/build.gradle.kts
 *
 * Module-level build file for the Cheapskate Android app.
 *
 * This module:
 *  - Targets Android API 34 (Android 14), minimum API 24 (Android 7.0 Nougat)
 *  - Uses Kotlin with coroutines for async server management
 *  - Ships the pre-compiled Go server binary as a raw asset (server-arm64)
 *
 * Binary placement (done outside Gradle, e.g. by a Makefile or CI step):
 *   android/app/src/main/assets/server-arm64
 *
 * The asset is extracted to the app's private files directory at first launch by
 * ServerManager.extractBinaryIfNeeded().
 */

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

android {
    namespace = "com.cheapskate.app"

    // Must match the Android Gradle Plugin version used in the root build.gradle.kts
    compileSdk = 34

    defaultConfig {
        applicationId = "com.cheapskate.app"

        // API 24 = Android 7.0 Nougat: first version with reliable process execution
        // from the app's private files directory without root.
        minSdk = 24

        targetSdk = 34

        versionCode = 1
        versionName = "0.1.0"

        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
    }

    buildTypes {
        release {
            // Minify and obfuscate for release builds.
            isMinifyEnabled = true
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro"
            )
        }
        debug {
            // Debug builds: no minification, easier to inspect logs.
            isMinifyEnabled = false
            applicationIdSuffix = ".debug"
            versionNameSuffix = "-debug"
        }
    }

    compileOptions {
        // Java 8 source/target compatibility is required by many AndroidX libraries.
        sourceCompatibility = JavaVersion.VERSION_1_8
        targetCompatibility = JavaVersion.VERSION_1_8
    }

    kotlinOptions {
        jvmTarget = "1.8"
    }

    // Keep assets directory in the default location; no special configuration needed.
    // The Go binary placed at src/main/assets/server-arm64 is included automatically.
}

dependencies {
    // AppCompat: provides AppCompatActivity (used by MainActivity) and the Material
    // theme required for the dark action bar.
    implementation("androidx.appcompat:appcompat:1.7.0")

    // Material Components: required for AlertDialog and Switch used in the settings UI.
    implementation("com.google.android.material:material:1.12.0")

    // Activity KTX: provides lifecycleScope extension on AppCompatActivity.
    implementation("androidx.activity:activity-ktx:1.9.0")

    // Lifecycle KTX: provides lifecycleScope and viewModelScope coroutine extensions.
    implementation("androidx.lifecycle:lifecycle-runtime-ktx:2.8.3")

    // Kotlin Coroutines for Android: Dispatchers.Main, Dispatchers.IO, etc.
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.8.1")

    // --- Test dependencies ---

    testImplementation("junit:junit:4.13.2")

    androidTestImplementation("androidx.test.ext:junit:1.2.1")
    androidTestImplementation("androidx.test.espresso:espresso-core:3.6.1")
}
