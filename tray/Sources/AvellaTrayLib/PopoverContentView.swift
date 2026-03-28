import SwiftUI

struct PopoverContentView: View {
    @ObservedObject var viewModel: TrayViewModel

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            headerSection
            Divider()
            recentFilesSection
            Divider()
            togglesSection
            Divider()
            rulesSection
            Divider()
            actionsSection
        }
        .frame(width: 320)
    }

    // MARK: - Header

    private var headerSection: some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack(spacing: 8) {
                Circle()
                    .fill(statusColor)
                    .frame(width: 8, height: 8)
                Text(viewModel.status)
                    .font(.system(size: 13, weight: .semibold))
                Spacer()
                if !viewModel.version.isEmpty {
                    Text("v\(viewModel.version)")
                        .font(.system(size: 11))
                        .foregroundColor(.secondary)
                }
            }

            if case .protocolMismatch = viewModel.connectionState {
                Text("Update tray or daemon to match")
                    .font(.system(size: 11))
                    .foregroundColor(.orange)
            } else {
                Text("Processed: \(viewModel.processed) files")
                    .font(.system(size: 12))
                    .foregroundColor(.secondary)
            }
        }
        .padding(12)
    }

    private var statusColor: Color {
        switch viewModel.connectionState {
        case .connected: return .green
        case .disconnected: return .secondary
        case .protocolMismatch: return .orange
        }
    }

    // MARK: - Recent Files

    private var recentFilesSection: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text("Recent Files")
                .font(.system(size: 11, weight: .semibold))
                .foregroundColor(.secondary)

            if viewModel.recentFiles.isEmpty {
                Text("(none)")
                    .font(.system(size: 12))
                    .foregroundColor(.secondary)
            } else {
                ForEach(Array(viewModel.recentFiles.enumerated()), id: \.offset) { _, file in
                    recentFileRow(file)
                }
            }
        }
        .padding(12)
    }

    private func recentFileRow(_ file: RecentFile) -> some View {
        VStack(alignment: .leading, spacing: 2) {
            HStack(spacing: 4) {
                if file.dryRun {
                    Text("dry-run")
                        .font(.system(size: 9, weight: .medium))
                        .foregroundColor(.orange)
                        .padding(.horizontal, 4)
                        .padding(.vertical, 1)
                        .background(Color.orange.opacity(0.12))
                        .cornerRadius(3)
                }
                Text(file.filename)
                    .font(.system(size: 12, weight: .medium))
                    .lineLimit(1)
            }
            HStack(spacing: 4) {
                Text(file.rule)
                    .font(.system(size: 10))
                    .foregroundColor(.secondary)
                Image(systemName: "arrow.right")
                    .font(.system(size: 8))
                    .foregroundColor(.secondary)
                Text(file.action)
                    .font(.system(size: 10))
                    .foregroundColor(.secondary)
                    .lineLimit(1)
            }
        }
        .padding(.vertical, 2)
    }

    // MARK: - Toggles

    private var togglesSection: some View {
        Grid(alignment: .leading, verticalSpacing: 8) {
            GridRow {
                Text("Dry-run mode")
                    .gridColumnAlignment(.leading)
                Toggle("", isOn: Binding(
                    get: { viewModel.dryRun },
                    set: { _ in viewModel.onToggleDryRun?() }
                ))
                .toggleStyle(.switch)
                .controlSize(.small)
                .labelsHidden()
                .gridColumnAlignment(.trailing)
                .disabled(!viewModel.isConnected)
            }

            GridRow {
                Text("Notifications")
                Toggle("", isOn: Binding(
                    get: { viewModel.notificationsEnabled },
                    set: { _ in
                        viewModel.onToggleNotifications?()
                        viewModel.notificationsEnabled = NotificationManager.shared.isEnabled
                    }
                ))
                .toggleStyle(.switch)
                .controlSize(.small)
                .labelsHidden()
            }
        }
        .padding(12)
    }

    // MARK: - Rules

    private var rulesSection: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text("Rules")
                .font(.system(size: 11, weight: .semibold))
                .foregroundColor(.secondary)

            if viewModel.rules.isEmpty {
                Text("(none)")
                    .font(.system(size: 12))
                    .foregroundColor(.secondary)
            } else {
                Grid(alignment: .leading, verticalSpacing: 4) {
                    ForEach(Array(viewModel.rules.enumerated()), id: \.offset) { _, rule in
                        GridRow {
                            Text(rule.name)
                                .font(.system(size: 12, weight: .medium))
                                .gridColumnAlignment(.leading)
                            Text(rule.actionType)
                                .font(.system(size: 11))
                                .foregroundColor(.secondary)
                                .gridColumnAlignment(.leading)
                        }
                    }
                }
            }
        }
        .padding(12)
    }

    // MARK: - Actions

    private var actionsSection: some View {
        VStack(spacing: 6) {
            Button(action: { viewModel.onOpenConfig?() }) {
                HStack {
                    Image(systemName: "doc.text")
                        .font(.system(size: 11))
                    Text("Open Config")
                        .font(.system(size: 12))
                    Spacer()
                }
            }
            .buttonStyle(.plain)
            .disabled(!viewModel.isConnected)

            Divider()

            Button(action: { viewModel.onQuit?() }) {
                HStack {
                    Image(systemName: "power")
                        .font(.system(size: 11))
                    Text("Quit")
                        .font(.system(size: 12))
                    Spacer()
                    Text("\u{2318}Q")
                        .font(.system(size: 11))
                        .foregroundColor(.secondary)
                }
            }
            .buttonStyle(.plain)
        }
        .padding(12)
    }
}
