---
name: implement-bubble-tea-tui
description: This skill guides Claude in implementing interactive Terminal User Interface (TUI) components and workflows using the `charmbracelet/bubbletea` framework, including models, views, and update functions.
---

## Implementing Terminal User Interfaces with `charmbracelet/bubbletea`

`ccb` relies heavily on `charmbracelet/bubbletea` for its interactive Terminal User Interface (TUI). Claude needs to understand the core concepts of this framework to effectively design and implement TUI components within the `internal/tui` package.

**Core Concepts (Model-View-Update - MVU Pattern):**

1.  **Model (`tea.Model`)**: Represents the state of your TUI application or component. It's a Go struct that holds all the data needed to render the view and react to user input.
    *   Must implement the `tea.Model` interface, which includes `Init()`, `Update(msg tea.Msg)`, and `View()` methods.

2.  **Init (`tea.Cmd`)**: The `Init()` method is called once at the start of the program. It returns an initial `tea.Cmd` (command) that can perform setup tasks (e.g., start a spinner, fetch initial data).
    *   `tea.Cmd` is a function that returns a `tea.Msg` (message).

3.  **Messages (`tea.Msg`)**: Events that happen in your application. These can be user input (key presses, mouse clicks), timer ticks, or results from asynchronous operations (e.g., data fetched from an API).
    *   Messages are typically empty structs or structs holding data related to the event.

4.  **Update (`tea.Msg`)**: The `Update(msg tea.Msg)` method is the heart of the application's logic. It receives messages, updates the `Model`'s state based on the message, and returns a new `Model` and an optional `tea.Cmd`.
    *   `switch msg := msg.(type)` is commonly used to handle different message types.
    *   Commands returned from `Update` are executed asynchronously and their results are sent back as new messages.

5.  **View (`string`)**: The `View()` method takes the current `Model`'s state and returns a string that represents the TUI to be rendered to the terminal.
    *   Use `charmbracelet/lipgloss` within `View()` to apply consistent styling (colors, borders, layouts) as defined in `internal/tui`.

**Typical Workflow:**

1.  Define a `Model` struct for your component (e.g., `WizardModel`, `SpinnerModel`).
2.  Implement `Init()` to set up initial state and commands.
3.  Implement `Update()` to react to `tea.Msg` (e.g., `tea.KeyMsg` for user input, custom messages for async results) and modify the `Model`.
4.  Implement `View()` to render the `Model`'s state as a string.
5.  Use `tea.Program` to run your top-level model.

**Claude's Task:**

When asked to create or modify a TUI component, Claude should:
*   Design the `Model` to hold necessary state.
*   Define relevant `Msg` types for user interactions or background tasks.
*   Implement `Update` logic to transition the `Model`'s state.
*   Craft the `View` method to render the UI, utilizing `lipgloss` for styling. Ensure consistency with existing `internal/tui` components.