import SwiftUI

public struct SettingsView: View {
    var viewModel: TrayViewModel

    public init(viewModel: TrayViewModel) {
        self.viewModel = viewModel
    }

    public var body: some View {
        Form {
            Section("General") {
                // Toggling flips the current value via the daemon-agnostic
                // NotificationManager; set-value is ignored, matching the popover.
                Toggle("Notifications", isOn: Binding(
                    get: { viewModel.notificationsEnabled },
                    set: { _ in viewModel.perform(.toggleNotifications) }
                ))
                Toggle("Launch at Login", isOn: Binding(
                    get: { viewModel.launchAtLoginEnabled },
                    set: { viewModel.setLaunchAtLogin($0) }
                ))
            }

            if let error = viewModel.launchAtLoginError {
                Section {
                    Text(error)
                        .foregroundStyle(.red)
                }
            }
        }
        .formStyle(.grouped)
        .frame(width: 420, height: 200)
    }
}
