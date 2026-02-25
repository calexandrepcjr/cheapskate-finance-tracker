# Gradle Wrapper JAR

The `gradle-wrapper.jar` binary is not committed to this repository.

To generate it, run the following from the `android/` directory (requires Gradle 8.7+ installed):

```bash
gradle wrapper --gradle-version=8.7
```

Or, if you have Android Studio, simply open the `android/` directory as a project and it will
auto-download the wrapper automatically.
