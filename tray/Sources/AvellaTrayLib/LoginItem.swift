import ServiceManagement

/// Abstraction over `SMAppService` so launch-at-login can be unit-tested
/// without registering a real login item (which requires a signed app bundle
/// and mutates system state).
@MainActor
protocol LoginItemControlling {
    var isEnabled: Bool { get }
    func setEnabled(_ enabled: Bool) throws
}

/// Production implementation backed by `SMAppService.mainApp`.
struct SMAppServiceLoginItem: LoginItemControlling {
    var isEnabled: Bool { SMAppService.mainApp.status == .enabled }

    func setEnabled(_ enabled: Bool) throws {
        if enabled {
            try SMAppService.mainApp.register()
        } else {
            try SMAppService.mainApp.unregister()
        }
    }
}
