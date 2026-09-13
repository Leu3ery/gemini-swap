import Cocoa
import SwiftUI

class AppDelegate: NSObject, NSApplicationDelegate {
    var window: NSWindow?
    var statusItem: NSStatusItem?
    var popover: NSPopover?
    let appState = AppState.shared

    func applicationDidFinishLaunching(_ notification: Notification) {
        setupStatusItem()
        setupWindow()
        setupMenu()
    }

    private func setupStatusItem() {
        statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
        if let button = statusItem?.button {
            let image = NSImage(systemSymbolName: "sparkles", accessibilityDescription: "Gemini Swap")
            image?.isTemplate = true
            button.image = image
            button.action = #selector(togglePopover)
            button.target = self
        }

        let popover = NSPopover()
        popover.contentSize = NSSize(width: 290, height: 360)
        popover.behavior = .transient
        popover.contentViewController = NSHostingController(rootView: MenuBarView(
            appState: appState,
            onOpenWindow: { [weak self] in
                self?.showMainWindow()
            },
            onQuit: {
                NSApp.terminate(nil)
            }
        ))
        self.popover = popover
    }

    private func setupWindow() {
        let mainWindow = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: 780, height: 600),
            styleMask: [.titled, .closable, .miniaturizable, .resizable],
            backing: .buffered,
            defer: false
        )
        mainWindow.center()
        mainWindow.title = "Gemini Swap"
        mainWindow.minSize = NSSize(width: 650, height: 500)
        mainWindow.isReleasedWhenClosed = false

        mainWindow.contentView = NSHostingView(rootView: MainWindowView(
            appState: appState,
            onToggleUpperBar: { [weak self] in
                self?.switchToMenuBarMode()
            }
        ))

        self.window = mainWindow
        mainWindow.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
    }

    private func setupMenu() {
        let mainMenu = NSMenu()
        
        let appMenuItem = NSMenuItem()
        mainMenu.addItem(appMenuItem)
        
        let appMenu = NSMenu()
        appMenuItem.submenu = appMenu
        
        appMenu.addItem(withTitle: "About Gemini Swap", action: #selector(NSApplication.orderFrontStandardAboutPanel(_:)), keyEquivalent: "")
        appMenu.addItem(NSMenuItem.separator())
        appMenu.addItem(withTitle: "Upper Bar Mode", action: #selector(switchToMenuBarMode), keyEquivalent: "m")
        appMenu.addItem(NSMenuItem.separator())
        appMenu.addItem(withTitle: "Hide Gemini Swap", action: #selector(NSApplication.hide(_:)), keyEquivalent: "h")
        appMenu.addItem(withTitle: "Hide Others", action: #selector(NSApplication.hideOtherApplications(_:)), keyEquivalent: "h")
            .keyEquivalentModifierMask = [.command, .option]
        appMenu.addItem(withTitle: "Show All", action: #selector(NSApplication.unhideAllApplications(_:)), keyEquivalent: "")
        appMenu.addItem(NSMenuItem.separator())
        appMenu.addItem(withTitle: "Quit Gemini Swap", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q")
        
        NSApp.mainMenu = mainMenu
    }

    @objc func togglePopover() {
        guard let button = statusItem?.button, let popover = popover else { return }
        if popover.isShown {
            popover.performClose(nil)
        } else {
            popover.show(relativeTo: button.bounds, of: button, preferredEdge: .minY)
            popover.contentViewController?.view.window?.makeKey()
        }
    }

    @objc func switchToMenuBarMode() {
        popover?.performClose(nil)
        window?.orderOut(nil)
        NSApp.setActivationPolicy(.accessory)
        appState.isMenuBarOnly = true
    }

    func showMainWindow() {
        popover?.performClose(nil)
        NSApp.setActivationPolicy(.regular)
        window?.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
        appState.isMenuBarOnly = false
    }
}
