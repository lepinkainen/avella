import AppKit
import SwiftUI

public final class AppDelegate: NSObject, NSApplicationDelegate {
    private var statusItem: NSStatusItem!
    private var popover: NSPopover!
    private var viewModel: TrayViewModel!
    private var socketClient: SocketClient!
    private let notificationManager = NotificationManager.shared

    public func applicationDidFinishLaunching(_ notification: Notification) {
        notificationManager.setup()

        // Create status bar item first, before changing activation policy.
        statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.squareLength)

        if let button = statusItem.button {
            if let iconImage = loadIcon() {
                iconImage.isTemplate = true
                button.image = iconImage
            } else {
                button.title = "A"
            }
            button.toolTip = "Avella — file automation daemon"
            button.action = #selector(togglePopover(_:))
            button.target = self
        }

        // Hide from Dock — must be after status item creation.
        NSApp.setActivationPolicy(.accessory)

        viewModel = TrayViewModel()

        popover = NSPopover()
        popover.contentSize = NSSize(width: 320, height: 480)
        popover.behavior = .transient
        popover.contentViewController = NSHostingController(
            rootView: PopoverContentView(viewModel: viewModel)
        )

        socketClient = SocketClient()

        socketClient.onStateUpdate = { [weak self] state in
            self?.viewModel.update(state: state)
            self?.notificationManager.handleStateUpdate(recentFiles: state.recentFiles)
        }

        socketClient.onConnectionChange = { [weak self] connected in
            if !connected {
                self?.viewModel.setDisconnected()
            }
        }

        socketClient.onProtocolMismatch = { [weak self] version in
            self?.viewModel.setProtocolMismatch(
                daemon: version, tray: supportedProtocolVersion
            )
        }

        viewModel.onToggleDryRun = { [weak self] in
            self?.socketClient.send(command: "toggle_dry_run")
        }

        viewModel.onToggleNotifications = {
            let mgr = NotificationManager.shared
            mgr.setEnabled(!mgr.isEnabled)
        }

        viewModel.onOpenConfig = { [weak self] in
            self?.socketClient.send(command: "open_config")
        }

        viewModel.onQuit = { [weak self] in
            self?.socketClient.send(command: "quit")
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.3) {
                NSApplication.shared.terminate(nil)
            }
        }

        socketClient.start()
    }

    public func applicationWillTerminate(_ notification: Notification) {
        socketClient.stop()
    }

    @objc private func togglePopover(_ sender: AnyObject?) {
        guard let button = statusItem.button else { return }
        if popover.isShown {
            popover.performClose(sender)
        } else {
            NSApp.activate(ignoringOtherApps: true)
            popover.show(relativeTo: button.bounds, of: button, preferredEdge: .minY)
            popover.contentViewController?.view.window?.makeKey()
        }
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
