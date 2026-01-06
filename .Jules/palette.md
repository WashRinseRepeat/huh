## 2024-05-23 - Copied! Feedback
**Learning:** Users can be confused by silent application exits after an action like "copy". A brief delay with visual feedback ("Copied!") reassures the user that the action was successful before the app closes.
**Action:** When implementing "copy to clipboard and quit" patterns in TUI apps, always add a `StateCopied` transition with a short timeout (e.g., 800ms) to display a success message before `tea.Quit`.

## 2025-05-27 - Loading Spinner Standardization
**Learning:** Custom ASCII animations (like a blinking robot) can feel jerky and slow if tick rates are high (e.g., 750ms). Replacing them with standard libraries (like `bubbles/spinner`) provides smoother (100ms ticks), more professional feedback that assures the user the application is active and not frozen.
**Action:** Prefer standard `bubbles/spinner` components for loading states over custom manual frame toggling unless the custom animation is a core brand asset and can be optimized for smoothness.
