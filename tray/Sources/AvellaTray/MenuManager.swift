import AppKit

/// Builds and updates the NSMenu for the status bar item.
final class MenuManager {
    private let statusItem: NSStatusItem
    private let menu = NSMenu()

    private let statusMenuItem = NSMenuItem(title: "Disconnected", action: nil, keyEquivalent: "")
    private let processedMenuItem = NSMenuItem(title: "Processed: 0 files", action: nil, keyEquivalent: "")
    private let recentMenuItem = NSMenuItem(title: "Recent", action: nil, keyEquivalent: "")
    private let dryRunMenuItem: NSMenuItem
    private let notificationsMenuItem: NSMenuItem
    private let rulesMenuItem = NSMenuItem(title: "Rules", action: nil, keyEquivalent: "")
    private let openConfigMenuItem: NSMenuItem
    private let quitMenuItem: NSMenuItem

    var onToggleDryRun: (() -> Void)?
    var onToggleNotifications: (() -> Void)?
    var onOpenConfig: (() -> Void)?
    var onQuit: (() -> Void)?

    private var connected = false

    init(statusItem: NSStatusItem) {
        self.statusItem = statusItem

        dryRunMenuItem = NSMenuItem(title: "Dry-run mode", action: nil, keyEquivalent: "")
        notificationsMenuItem = NSMenuItem(title: "Notifications", action: nil, keyEquivalent: "")
        openConfigMenuItem = NSMenuItem(title: "Open Config", action: nil, keyEquivalent: "")
        quitMenuItem = NSMenuItem(title: "Quit", action: nil, keyEquivalent: "q")

        buildMenu()
        statusItem.menu = menu
        applyDisconnectedState()
    }

    func update(state: AppState) {
        connected = true
        statusMenuItem.title = state.status
        statusMenuItem.isEnabled = false
        processedMenuItem.title = "Processed: \(state.processed) files"
        processedMenuItem.isEnabled = false

        // Rebuild recent files submenu.
        let recentSubmenu = NSMenu()
        if state.recentFiles.isEmpty {
            let item = NSMenuItem(title: "(none)", action: nil, keyEquivalent: "")
            item.isEnabled = false
            recentSubmenu.addItem(item)
        } else {
            for file in state.recentFiles {
                let prefix = file.dryRun ? "[dry-run] " : ""
                let title = "\(prefix)\(file.filename) \u{2192} \(file.action)"
                let item = NSMenuItem(title: title, action: nil, keyEquivalent: "")
                item.isEnabled = false
                recentSubmenu.addItem(item)
            }
        }
        recentMenuItem.submenu = recentSubmenu
        recentMenuItem.isEnabled = true

        dryRunMenuItem.state = state.dryRun ? .on : .off
        dryRunMenuItem.isEnabled = true
        dryRunMenuItem.target = self
        dryRunMenuItem.action = #selector(dryRunClicked)

        notificationsMenuItem.state = NotificationManager.shared.isEnabled ? .on : .off
        notificationsMenuItem.isEnabled = true
        notificationsMenuItem.target = self
        notificationsMenuItem.action = #selector(notificationsClicked)

        // Rebuild rules submenu.
        let rulesSubmenu = NSMenu()
        for rule in state.rules {
            let item = NSMenuItem(title: "\(rule.name): \(rule.actionType)", action: nil, keyEquivalent: "")
            item.isEnabled = false
            rulesSubmenu.addItem(item)
        }
        if state.rules.isEmpty {
            let item = NSMenuItem(title: "(none)", action: nil, keyEquivalent: "")
            item.isEnabled = false
            rulesSubmenu.addItem(item)
        }
        rulesMenuItem.submenu = rulesSubmenu
        rulesMenuItem.isEnabled = true

        openConfigMenuItem.isEnabled = true
        openConfigMenuItem.target = self
        openConfigMenuItem.action = #selector(openConfigClicked)

        quitMenuItem.target = self
        quitMenuItem.action = #selector(quitClicked)
    }

    func setDisconnected() {
        connected = false
        applyDisconnectedState()
    }

    // MARK: - Private

    private func buildMenu() {
        menu.addItem(statusMenuItem)
        menu.addItem(processedMenuItem)
        menu.addItem(recentMenuItem)
        menu.addItem(NSMenuItem.separator())
        menu.addItem(dryRunMenuItem)
        menu.addItem(notificationsMenuItem)
        menu.addItem(NSMenuItem.separator())
        menu.addItem(rulesMenuItem)
        menu.addItem(openConfigMenuItem)
        menu.addItem(NSMenuItem.separator())
        menu.addItem(quitMenuItem)
    }

    private func applyDisconnectedState() {
        statusMenuItem.title = "Disconnected"
        statusMenuItem.isEnabled = false
        processedMenuItem.title = "Processed: 0 files"
        processedMenuItem.isEnabled = false
        recentMenuItem.isEnabled = false
        recentMenuItem.submenu = nil
        dryRunMenuItem.state = .off
        dryRunMenuItem.isEnabled = false
        rulesMenuItem.isEnabled = false
        rulesMenuItem.submenu = nil
        openConfigMenuItem.isEnabled = false

        // Notifications toggle works even when disconnected.
        notificationsMenuItem.state = NotificationManager.shared.isEnabled ? .on : .off
        notificationsMenuItem.isEnabled = true
        notificationsMenuItem.target = self
        notificationsMenuItem.action = #selector(notificationsClicked)

        // Quit is always available.
        quitMenuItem.isEnabled = true
        quitMenuItem.target = self
        quitMenuItem.action = #selector(quitClicked)
    }

    @objc private func dryRunClicked() {
        onToggleDryRun?()
    }

    @objc private func notificationsClicked() {
        onToggleNotifications?()
    }

    @objc private func openConfigClicked() {
        onOpenConfig?()
    }

    @objc private func quitClicked() {
        onQuit?()
    }
}
