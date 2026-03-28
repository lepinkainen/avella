import Foundation

/// Observable view model that bridges the imperative socket callbacks to SwiftUI's reactive model.
final class TrayViewModel: ObservableObject {
    enum ConnectionState {
        case disconnected
        case connected
        case protocolMismatch(daemon: Int, tray: Int)
    }

    @Published var connectionState: ConnectionState = .disconnected
    @Published var status: String = "Disconnected"
    @Published var processed: Int = 0
    @Published var dryRun: Bool = false
    @Published var notificationsEnabled: Bool = NotificationManager.shared.isEnabled
    @Published var recentFiles: [RecentFile] = []
    @Published var rules: [RuleInfo] = []
    @Published var version: String = ""

    var isConnected: Bool {
        if case .connected = connectionState { return true }
        return false
    }

    // Action callbacks — set by AppDelegate, invoked by SwiftUI views.
    var onToggleDryRun: (() -> Void)?
    var onToggleNotifications: (() -> Void)?
    var onOpenConfig: (() -> Void)?
    var onQuit: (() -> Void)?

    func update(state: AppState) {
        connectionState = .connected
        status = state.status
        processed = state.processed
        dryRun = state.dryRun
        recentFiles = state.recentFiles
        rules = state.rules
        version = state.version
        notificationsEnabled = NotificationManager.shared.isEnabled
    }

    func setDisconnected() {
        connectionState = .disconnected
        status = "Disconnected"
        processed = 0
        dryRun = false
        recentFiles = []
        rules = []
        notificationsEnabled = NotificationManager.shared.isEnabled
    }

    func setProtocolMismatch(daemon: Int, tray: Int) {
        connectionState = .protocolMismatch(daemon: daemon, tray: tray)
        status = "Protocol mismatch (daemon v\(daemon), tray v\(tray))"
    }
}
