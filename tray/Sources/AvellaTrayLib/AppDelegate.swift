import AppKit

public final class AppDelegate: NSObject, NSApplicationDelegate {
    private var statusItem: NSStatusItem!
    private var menuManager: MenuManager!
    private var socketClient: SocketClient!
    private let notificationManager = NotificationManager.shared

    public func applicationDidFinishLaunching(_ notification: Notification) {
        notificationManager.setup()

        statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.squareLength)

        if let button = statusItem.button {
            if let iconImage = loadIcon() {
                iconImage.isTemplate = true
                button.image = iconImage
            } else {
                button.title = "A"
            }
            button.toolTip = "Avella — file automation daemon"
        }

        menuManager = MenuManager(statusItem: statusItem)
        socketClient = SocketClient()

        socketClient.onStateUpdate = { [weak self] state in
            self?.menuManager.update(state: state)
            self?.notificationManager.handleStateUpdate(recentFiles: state.recentFiles)
        }

        socketClient.onConnectionChange = { [weak self] connected in
            if !connected {
                self?.menuManager.setDisconnected()
            }
        }

        socketClient.onProtocolMismatch = { [weak self] version in
            self?.menuManager.setProtocolMismatch(
                daemon: version, tray: supportedProtocolVersion
            )
        }

        menuManager.onToggleDryRun = { [weak self] in
            self?.socketClient.send(command: "toggle_dry_run")
        }

        menuManager.onToggleNotifications = {
            let mgr = NotificationManager.shared
            mgr.setEnabled(!mgr.isEnabled)
        }

        menuManager.onOpenConfig = { [weak self] in
            self?.socketClient.send(command: "open_config")
        }

        menuManager.onQuit = { [weak self] in
            self?.socketClient.send(command: "quit")
            // Give the command a moment to be sent before exiting.
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.3) {
                NSApplication.shared.terminate(nil)
            }
        }

        socketClient.start()
    }

    public func applicationWillTerminate(_ notification: Notification) {
        socketClient.stop()
    }

    private func loadIcon() -> NSImage? {
        guard let url = Bundle.module.url(forResource: "icon", withExtension: "png"),
              let image = NSImage(contentsOf: url) else {
            return nil
        }
        image.size = NSSize(width: 18, height: 18)
        return image
    }
}
