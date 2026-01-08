## 2024-05-23 - Copied! Feedback
**Learning:** Users can be confused by silent application exits after an action like "copy". A brief delay with visual feedback ("Copied!") reassures the user that the action was successful before the app closes.
**Action:** When implementing "copy to clipboard and quit" patterns in TUI apps, always add a `StateCopied` transition with a short timeout (e.g., 800ms) to display a success message before `tea.Quit`.

## 2025-05-27 - Delight over Standardization
**Learning:** While standard components (like spinners) are safe, users of this tool value "delight" and "cuteness" (like the robot mascot). Replacing unique personality with generic efficiency can be perceived as a downgrade.
**Action:** When enhancing UI, preserve the core "personality" of the tool. Use TUI libraries (Lipgloss) to polish existing unique assets (add color, smooth animation) rather than replacing them with generic ones.
