## 2024-05-23 - Copied! Feedback
**Learning:** Users can be confused by silent application exits after an action like "copy". A brief delay with visual feedback ("Copied!") reassures the user that the action was successful before the app closes.
**Action:** When implementing "copy to clipboard and quit" patterns in TUI apps, always add a `StateCopied` transition with a short timeout (e.g., 800ms) to display a success message before `tea.Quit`.
