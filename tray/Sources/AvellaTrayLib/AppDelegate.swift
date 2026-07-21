import AppKit
import SwiftUI

@MainActor
public final class AppDelegate: NSObject, NSApplicationDelegate {
    /// Shared view model, consumed by the SwiftUI scenes via the app delegate
    /// adaptor. Created eagerly so scene bodies can reference it immediately.
    public let viewModel = TrayViewModel()

    private var socketClient: SocketClient!
    private let notificationManager = NotificationManager.shared
    private var eventTask: Task<Void, Never>?

    public func applicationDidFinishLaunching(_ notification: Notification) {
        notificationManager.setup()

        // Hide from the Dock — this is a menu bar accessory app.
        NSApp.setActivationPolicy(.accessory)

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
}
