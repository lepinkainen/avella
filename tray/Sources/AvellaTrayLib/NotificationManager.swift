import AppKit
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

  // MARK: Lifecycle

  override init() {
    notificationsEnabled = UserDefaults.standard.object(forKey: "notificationsEnabled") as? Bool ?? true
    super.init()
  }

  // MARK: Internal

  static let shared = NotificationManager()

  /// Key under which the destination file path is stashed in a
  /// notification's `userInfo`, read back when the banner is tapped.
  nonisolated static let filePathUserInfoKey = "filePath"

  var lastSeenFiles = [RecentFile]()
  var firstUpdate = true
  var setupDone = false

  private(set) var notificationsEnabled: Bool {
    didSet {
      UserDefaults.standard.set(notificationsEnabled, forKey: "notificationsEnabled")
    }
  }

  var isEnabled: Bool {
    notificationsEnabled
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

  /// Show notifications even when the app is in the foreground.
  nonisolated func userNotificationCenter(
    _: UNUserNotificationCenter,
    willPresent _: UNNotification,
    withCompletionHandler completionHandler: @escaping (UNNotificationPresentationOptions) -> Void,
  ) {
    completionHandler([.banner, .sound])
  }

  /// Tapping a banner reveals the processed file in Finder.
  nonisolated func userNotificationCenter(
    _: UNUserNotificationCenter,
    didReceive response: UNNotificationResponse,
    withCompletionHandler completionHandler: @escaping () -> Void,
  ) {
    let path = response.notification.request.content.userInfo[Self.filePathUserInfoKey] as? String
    completionHandler()

    guard let path, !path.isEmpty else { return }
    // Delegate callbacks are nonisolated; NSWorkspace is main-actor bound.
    Task { @MainActor in
      NSWorkspace.shared.activateFileViewerSelecting([URL(fileURLWithPath: path)])
    }
  }

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

  // MARK: Private

  private func postNotification(for file: RecentFile) {
    let content = UNMutableNotificationContent()
    content.title = file.dryRun ? "[dry-run] Avella: \(file.rule)" : "Avella: \(file.rule)"
    content.body = "\(file.filename) \u{2192} \(file.action)"
    content.sound = .default
    // Carry the destination path so a tap can reveal it in Finder.
    content.userInfo = [Self.filePathUserInfoKey: file.action]

    let request = UNNotificationRequest(
      identifier: UUID().uuidString,
      content: content,
      trigger: nil,
    )
    UNUserNotificationCenter.current().add(request)
  }

}
