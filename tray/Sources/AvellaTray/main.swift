import AvellaTrayLib
import SwiftUI

@main
struct AvellaTrayApp: App {
  @NSApplicationDelegateAdaptor(AppDelegate.self) var appDelegate

  var body: some Scene {
    MenuBarExtra {
      PopoverContentView(viewModel: appDelegate.viewModel)
    } label: {
      MenuBarLabel(viewModel: appDelegate.viewModel)
    }
    .menuBarExtraStyle(.window)

    Settings {
      SettingsView(viewModel: appDelegate.viewModel)
    }
  }
}
