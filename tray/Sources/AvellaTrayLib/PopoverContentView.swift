import SwiftUI

public struct PopoverContentView: View {
    var viewModel: TrayViewModel

    public init(viewModel: TrayViewModel) {
        self.viewModel = viewModel
    }

    public var body: some View {
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
                        .foregroundStyle(.secondary)
                }
            }

            if case .protocolMismatch = viewModel.connectionState {
                Text("Update tray or daemon to match")
                    .font(.system(size: 11))
                    .foregroundStyle(.orange)
            } else {
                Text("Processed: \(viewModel.processed) files")
                    .font(.system(size: 12))
                    .foregroundStyle(.secondary)
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
                .foregroundStyle(.secondary)

            if viewModel.recentFiles.isEmpty {
                Text("(none)")
                    .font(.system(size: 12))
                    .foregroundStyle(.secondary)
            } else {
                // Positional identity: RecentFile has no collision-proof id
                // (daemon timestamps are 1s granularity), and the list is
                // replaced wholesale on each state push, so offset is safe.
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
                        .foregroundStyle(.orange)
                        .padding(.horizontal, 4)
                        .padding(.vertical, 1)
                        .background(Color.orange.opacity(0.12))
                        .clipShape(RoundedRectangle(cornerRadius: 3))
                }
                Text(file.filename)
                    .font(.system(size: 12, weight: .medium))
                    .lineLimit(1)
            }
            HStack(spacing: 4) {
                Text(file.rule)
                    .font(.system(size: 10))
                    .foregroundStyle(.secondary)
                Image(systemName: "arrow.right")
                    .font(.system(size: 8))
                    .foregroundStyle(.secondary)
                Text(file.action)
                    .font(.system(size: 10))
                    .foregroundStyle(.secondary)
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
                    set: { _ in viewModel.perform(.toggleDryRun) }
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
                    set: { _ in viewModel.perform(.toggleNotifications) }
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
                .foregroundStyle(.secondary)

            if viewModel.rules.isEmpty {
                Text("(none)")
                    .font(.system(size: 12))
                    .foregroundStyle(.secondary)
            } else {
                Grid(alignment: .leading, verticalSpacing: 4) {
                    ForEach(viewModel.rules) { rule in
                        GridRow {
                            Text(rule.name)
                                .font(.system(size: 12, weight: .medium))
                                .gridColumnAlignment(.leading)
                            Text(rule.actionType)
                                .font(.system(size: 11))
                                .foregroundStyle(.secondary)
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
            Button(action: { viewModel.perform(.openConfig) }) {
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

            SettingsLink {
                HStack {
                    Image(systemName: "gearshape")
                        .font(.system(size: 11))
                    Text("Settings\u{2026}")
                        .font(.system(size: 12))
                    Spacer()
                }
            }
            .buttonStyle(.plain)

            Divider()

            Button(action: { viewModel.perform(.quit) }) {
                HStack {
                    Image(systemName: "power")
                        .font(.system(size: 11))
                    Text("Quit")
                        .font(.system(size: 12))
                    Spacer()
                    Text("\u{2318}Q")
                        .font(.system(size: 11))
                        .foregroundStyle(.secondary)
                }
            }
            .buttonStyle(.plain)
            .keyboardShortcut("q")
        }
        .padding(12)
    }
}
