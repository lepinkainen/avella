import SwiftUI

/// The menu bar item's label. Observes `TrayViewModel` (which is `@Observable`,
/// so reading its properties here registers for updates) to dim the icon while
/// disconnected and show a dry-run badge.
public struct MenuBarLabel: View {

  // MARK: Lifecycle

  public init(viewModel: TrayViewModel) {
    self.viewModel = viewModel
  }

  // MARK: Public

  public var body: some View {
    HStack(spacing: 2) {
      Image(nsImage: Self.icon)
        .opacity(viewModel.isConnected ? 1.0 : 0.4)
      if viewModel.dryRun {
        Image(systemName: "d.circle")
          .imageScale(.small)
      }
    }
    .accessibilityLabel("Avella — \(viewModel.status)")
  }

  // MARK: Internal

  let viewModel: TrayViewModel

  // MARK: Private

  /// Template menu bar icon; the system tints it to match the menu bar.
  private static let icon: NSImage = {
    guard
      let url = Bundle.module.url(forResource: "icon", withExtension: "png"),
      let image = NSImage(contentsOf: url)
    else {
      return NSImage(systemSymbolName: "tray", accessibilityDescription: "Avella") ?? NSImage()
    }
    image.size = NSSize(width: 18, height: 18)
    image.isTemplate = true
    return image
  }()

}
