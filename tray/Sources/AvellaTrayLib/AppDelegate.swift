import AppKit
import SwiftUI

@MainActor
public final class AppDelegate: NSObject, NSApplicationDelegate {
    private var statusItem: NSStatusItem!
    private var popover: NSPopover!
    private var viewModel: TrayViewModel!
    private var socketClient: SocketClient!
    private let notificationManager = NotificationManager.shared
    private var eventTask: Task<Void, Never>?

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

        viewModel.onAction = { [weak self] action in
            guard let self else { return }
            switch action {
            case .toggleDryRun:
                Task { await self.socketClient.send(.toggleDryRun) }
            case .toggleNotifications:
                let mgr = NotificationManager.shared
                mgr.setEnabled(!mgr.isEnabled)
            case .openConfig:
                Task { await self.socketClient.send(.openConfig) }
            case .quit:
                let client = self.socketClient!
                Task {
                    // Await the flush so the daemon actually receives the quit
                    // command, but bound it: a hung socket must not block quit.
                    await withTaskGroup(of: Void.self) { group in
                        group.addTask { _ = await client.send(.quit) }
                        group.addTask { try? await Task.sleep(nanoseconds: 1_000_000_000) }
                        _ = await group.next()
                        group.cancelAll()
                    }
                    NSApplication.shared.terminate(nil)
                }
            }
        }

        let client = socketClient!
        eventTask = Task { [weak self] in
            for await event in client.events {
                guard let self else { return }
                switch event {
                case .connected:
                    break
                case .disconnected:
                    self.viewModel.setDisconnected()
                case .protocolMismatch(let daemonVersion):
                    self.viewModel.setProtocolMismatch(
                        daemon: daemonVersion, tray: supportedProtocolVersion
                    )
                case .state(let state):
                    self.viewModel.update(state: state)
                    self.notificationManager.handleStateUpdate(recentFiles: state.recentFiles)
                }
            }
        }

        Task { await socketClient.start() }
    }

    public func applicationWillTerminate(_ notification: Notification) {
        eventTask?.cancel()
        eventTask = nil
        Task { await socketClient.stop() }
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
