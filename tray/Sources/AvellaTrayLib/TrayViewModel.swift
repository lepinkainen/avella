import Foundation
import Observation

// MARK: - TrayAction

/// Actions a user can trigger from the tray UI. Wired up by AppDelegate.
enum TrayAction {
  case toggleDryRun
  case toggleNotifications
  case openConfig
  case quit
}

// MARK: - TrayViewModel

/// Observable view model that bridges socket events to SwiftUI's reactive model.
@MainActor
@Observable
public final class TrayViewModel {

  // MARK: Lifecycle

  init(loginItem: any LoginItemControlling = SMAppServiceLoginItem()) {
    self.loginItem = loginItem
    launchAtLoginEnabled = loginItem.isEnabled
  }

  // MARK: Internal

  enum ConnectionState {
    case disconnected
    case connected
    case protocolMismatch(daemon: Int, tray: Int)
  }

  var connectionState = ConnectionState.disconnected
  var processed = 0
  var dryRun = false
  var recentFiles = [RecentFile]()
  var rules = [RuleInfo]()
  var version = ""

  /// Whether the app is registered to launch at login. Reflects the
  /// authoritative `SMAppService` status, re-read after every toggle.
  var launchAtLoginEnabled: Bool
  /// Last non-fatal launch-at-login error, surfaced in Settings.
  var launchAtLoginError: String?

  /// Set by AppDelegate, invoked by SwiftUI views via `perform(_:)`.
  var onAction: ((TrayAction) -> Void)?

  /// Single source of truth is the @Observable NotificationManager;
  /// observation tracking flows through this computed access.
  var notificationsEnabled: Bool {
    NotificationManager.shared.isEnabled
  }

  var status: String {
    switch connectionState {
    case .disconnected:
      "Disconnected"
    case .connected:
      daemonStatus
    case .protocolMismatch(let daemon, let tray):
      "Protocol mismatch (daemon v\(daemon), tray v\(tray))"
    }
  }

  var isConnected: Bool {
    if case .connected = connectionState {
      return true
    }
    return false
  }

  func perform(_ action: TrayAction) {
    onAction?(action)
  }

  func update(state: AppState) {
    connectionState = .connected
    daemonStatus = state.status
    processed = state.processed
    dryRun = state.dryRun
    recentFiles = state.recentFiles
    rules = state.rules
    version = state.version
  }

  func setDisconnected() {
    connectionState = .disconnected
    daemonStatus = "Disconnected"
    processed = 0
    dryRun = false
    recentFiles = []
    rules = []
  }

  func setProtocolMismatch(daemon: Int, tray: Int) {
    connectionState = .protocolMismatch(daemon: daemon, tray: tray)
  }

  /// Registers or unregisters the app as a login item. Failures are
  /// non-fatal: the error is surfaced and the flag is reconciled with the
  /// service's authoritative status rather than the requested value.
  func setLaunchAtLogin(_ enabled: Bool) {
    do {
      try loginItem.setEnabled(enabled)
      launchAtLoginError = nil
    } catch {
      launchAtLoginError = "Launch at Login: \(error.localizedDescription)"
    }
    launchAtLoginEnabled = loginItem.isEnabled
  }

  // MARK: Private

  /// Backing service for launch-at-login; injectable for testing.
  @ObservationIgnored private let loginItem: any LoginItemControlling

  /// Raw status string reported by the daemon while connected.
  private var daemonStatus = "Disconnected"

}
