import Foundation
import UserNotifications

/// Posts native macOS notifications when new files are processed.
final class NotificationManager: NSObject, UNUserNotificationCenterDelegate {
    static let shared = NotificationManager()

    private var notificationsEnabled: Bool
    private var lastSeenFiles: [RecentFile] = []
    private var firstUpdate = true
    private var setupDone = false

    var isEnabled: Bool { notificationsEnabled }

    override init() {
        notificationsEnabled = UserDefaults.standard.object(forKey: "notificationsEnabled") as? Bool ?? true
        super.init()
    }

    /// Must be called after NSApplication is running and bundle is available.
    func setup() {
        guard !setupDone else { return }
        setupDone = true
        UNUserNotificationCenter.current().delegate = self
        UNUserNotificationCenter.current().requestAuthorization(options: [.alert, .sound]) { _, error in
            if let error = error {
                print("Notification auth error: \(error)")
            }
        }
    }

    func setEnabled(_ enabled: Bool) {
        notificationsEnabled = enabled
        UserDefaults.standard.set(enabled, forKey: "notificationsEnabled")
    }

    func handleStateUpdate(recentFiles: [RecentFile]) {
        // Skip notifications on the very first update to avoid flooding
        // from processExistingFiles at startup.
        if firstUpdate {
            firstUpdate = false
            lastSeenFiles = recentFiles
            return
        }

        guard notificationsEnabled, setupDone else {
            lastSeenFiles = recentFiles
            return
        }

        let newCount = countNewFiles(current: recentFiles, previous: lastSeenFiles)
        for file in recentFiles.prefix(newCount) {
            postNotification(for: file)
        }

        lastSeenFiles = recentFiles
    }

    // MARK: - UNUserNotificationCenterDelegate

    /// Show notifications even when the app is in the foreground.
    func userNotificationCenter(
        _ center: UNUserNotificationCenter,
        willPresent notification: UNNotification,
        withCompletionHandler completionHandler: @escaping (UNNotificationPresentationOptions) -> Void
    ) {
        completionHandler([.banner, .sound])
    }

    // MARK: - Private

    private func countNewFiles(current: [RecentFile], previous: [RecentFile]) -> Int {
        guard let firstOld = previous.first else { return current.count }
        for (i, file) in current.enumerated() {
            if file.filename == firstOld.filename
                && file.time == firstOld.time
                && file.rule == firstOld.rule
            {
                return i
            }
        }
        return current.count
    }

    private func postNotification(for file: RecentFile) {
        let content = UNMutableNotificationContent()
        content.title = file.dryRun ? "[dry-run] Avella: \(file.rule)" : "Avella: \(file.rule)"
        content.body = "\(file.filename) \u{2192} \(file.action)"
        content.sound = .default

        let request = UNNotificationRequest(
            identifier: UUID().uuidString,
            content: content,
            trigger: nil
        )
        UNUserNotificationCenter.current().add(request)
    }
}
