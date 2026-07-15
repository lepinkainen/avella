import Foundation
import Observation
import UserNotifications

/// Posts native macOS notifications when new files are processed.
///
/// `@Observable` so SwiftUI views can read `isEnabled` (directly or through
/// `TrayViewModel`) without a manually synced mirror property.
@MainActor
@Observable
final class NotificationManager: NSObject, UNUserNotificationCenterDelegate {
    static let shared = NotificationManager()

    private(set) var notificationsEnabled: Bool {
        didSet {
            UserDefaults.standard.set(notificationsEnabled, forKey: "notificationsEnabled")
        }
    }
    var lastSeenFiles: [RecentFile] = []
    var firstUpdate = true
    var setupDone = false

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
        Task {
            do {
                _ = try await UNUserNotificationCenter.current().requestAuthorization(options: [.alert, .sound])
            } catch {
                print("Notification auth error: \(error)")
            }
        }
    }

    func setEnabled(_ enabled: Bool) {
        notificationsEnabled = enabled
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
    nonisolated func userNotificationCenter(
        _ center: UNUserNotificationCenter,
        willPresent notification: UNNotification,
        withCompletionHandler completionHandler: @escaping (UNNotificationPresentationOptions) -> Void
    ) {
        completionHandler([.banner, .sound])
    }

    // MARK: - Private

    func countNewFiles(current: [RecentFile], previous: [RecentFile]) -> Int {
        guard let firstOld = previous.first else { return current.count }
        // Match on filename+time+rule only: the daemon legitimately re-emits
        // the head entry with a flipped dryRun (dry-run toggle then real
        // execution) or a changed action while these three fields stay the
        // same — that must not be treated as a new file.
        return current.firstIndex {
            $0.filename == firstOld.filename && $0.time == firstOld.time && $0.rule == firstOld.rule
        } ?? current.count
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
