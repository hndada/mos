Muang OS, aiming for Mobile
===
# Terms
Program: executable jobs
Application (app): Visible Program. Has UI.

# Interaction Layers
1. User: Screen and touchscreen (and a few physical buttons)
2. App Developer: Defines UI components, background logic, requests to the system, and handling of system requests (interrupts)
3. System (Middleware): Handles requests from apps (e.g., drawing windows), listens for system operation requests, and system interrupts
4. OS: Writes and delivers sensor values, creates processes, etc.