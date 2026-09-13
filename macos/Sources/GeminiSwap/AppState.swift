import Foundation
import Combine
import AppKit
import UniformTypeIdentifiers

class AppState: ObservableObject {
    @Published var accounts: [Account] = []
    @Published var activeAccountID: String = ""
    @Published var proxyStats: ProxyStats? = nil
    @Published var isRefreshing: Bool = false
    @Published var isMenuBarOnly: Bool = false
    @Published var lastRefreshed: Date? = nil
    @Published var errorMessage: String? = nil
    @Published var toastMessage: String? = nil
    @Published var switchingAccountID: String? = nil
    @Published var switchingStatusText: String? = nil
    @Published var switchStartedAt: Date? = nil

    private var fileWatcherSource: DispatchSourceFileSystemObject?
    private var fileDescriptor: Int32 = -1
    private var statsTimer: Timer?

    static let shared = AppState()

    var activeAccount: Account? {
        accounts.first(where: { $0.id == activeAccountID }) ?? accounts.first
    }

    var isSwitchingAccount: Bool {
        switchingAccountID != nil
    }

    var switchingAccount: Account? {
        guard let id = switchingAccountID else { return nil }
        return accounts.first(where: { $0.id == id })
    }

    var baseDir: URL {
        FileManager.default.homeDirectoryForCurrentUser.appendingPathComponent(".gemini-swap")
    }

    var accountsFileURL: URL {
        baseDir.appendingPathComponent("accounts.json")
    }

    var statsFileURL: URL {
        baseDir.appendingPathComponent("stats.json")
    }

    init() {
        loadData()
        startWatchingFiles()
        startStatsTimer()
    }

    deinit {
        stopWatchingFiles()
        statsTimer?.invalidate()
    }

    func loadData() {
        guard FileManager.default.fileExists(atPath: accountsFileURL.path) else {
            // Trigger CLI to auto-import existing account if needed
            runCLICommand(["list"])
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.5) { [weak self] in
                self?.readAccountsFile()
            }
            return
        }
        readAccountsFile()
        readStatsFile()
    }

    private func readAccountsFile() {
        do {
            let data = try Data(contentsOf: accountsFileURL)
            let store = try JSONDecoder().decode(StoreData.self, from: data)
            DispatchQueue.main.async {
                self.accounts = store.accounts
                self.activeAccountID = store.activeAccountID
                self.lastRefreshed = Date()
            }
        } catch {
            print("Failed to read accounts file: \(error)")
        }
    }

    private func readStatsFile() {
        guard FileManager.default.fileExists(atPath: statsFileURL.path) else { return }
        do {
            let data = try Data(contentsOf: statsFileURL)
            let stats = try JSONDecoder().decode(ProxyStats.self, from: data)
            DispatchQueue.main.async {
                self.proxyStats = stats
            }
        } catch {
            print("Failed to read stats file: \(error)")
        }
    }

    func switchAccount(to id: String) {
        guard !isSwitchingAccount else { return }
        guard let target = accounts.first(where: { $0.id == id }) else { return }

        // Legacy accounts were authorized with Gemini CLI's OAuth client.
        // Antigravity cannot refresh those tokens. Upgrade the selected
        // account interactively before attempting the actual IDE switch.
        if target.type == .oauth && target.oauth?.client != "antigravity" {
            guard let email = target.email, !email.isEmpty else {
                showToast("This account must be authorized for Antigravity first")
                return
            }
            switchingAccountID = id
            switchingStatusText = "Waiting for Google authorization…"
            switchStartedAt = Date()
            DispatchQueue.global(qos: .userInitiated).async { [weak self] in
                guard let self = self else { return }
                let output = self.runCLICommand(["login", "--ide", "--email", email])
                DispatchQueue.main.async {
                    self.finishSwitch(target: target, output: output)
                }
            }
            return
        }

        switchingAccountID = id
        switchingStatusText = "Switching Antigravity…"
        switchStartedAt = Date()
        DispatchQueue.global(qos: .userInitiated).async { [weak self] in
            guard let self = self else { return }
            let output = self.runCLICommand(["switch", id])
            DispatchQueue.main.async {
                self.finishSwitch(target: target, output: output)
            }
        }
    }

    private func finishSwitch(target: Account, output: String) {
        // CLI is the single source of truth: it updates accounts.json,
        // ~/.gemini/* and the Antigravity keychain + reloads the IDE.
        // (Previous version additionally rewrote those files here from the
        // possibly stale in-memory cache, which could overwrite fresh tokens.)
        // Verify persistence synchronously instead of trusting the toast:
        // if the CLI binary is missing or the switch failed, accounts.json
        // still holds the old activeAccountID and we must revert + show error.
        do {
            let data = try Data(contentsOf: accountsFileURL)
            let store = try JSONDecoder().decode(StoreData.self, from: data)
            DispatchQueue.main.async {
                self.switchingAccountID = nil
                self.switchingStatusText = nil
                self.switchStartedAt = nil
                self.accounts = store.accounts
                self.activeAccountID = store.activeAccountID
                self.lastRefreshed = Date()
                let confirmed = output.contains("Active account switched to:") || output.contains("Successfully added Google account:")
                if store.activeAccountID == target.id && confirmed {
                    let warnings = output.components(separatedBy: "\n")
                        .map { $0.trimmingCharacters(in: .whitespaces) }
                        .filter { $0.hasPrefix("Warning:") }
                    if let first = warnings.first {
                        let short = String(first.dropFirst("Warning:".count).trimmingCharacters(in: .whitespaces).prefix(140))
                        self.showToast("Switched to \(target.name), but: \(short)")
                    } else {
                        self.showToast("Switched to \(target.name) (Antigravity & CLI synced)")
                    }
                } else {
                    let detail = output.trimmingCharacters(in: .whitespacesAndNewlines)
                    self.showToast(detail.isEmpty ? "Switch failed: CLI did not update accounts.json" : "Switch failed: \(String(detail.prefix(180)))")
                }
            }
        } catch {
            // CLI failed and/or file unreadable
            DispatchQueue.main.async {
                self.switchingAccountID = nil
                self.switchingStatusText = nil
                self.switchStartedAt = nil
                let detail = output.trimmingCharacters(in: .whitespacesAndNewlines)
                self.showToast(detail.isEmpty ? "Switch failed" : "Switch failed: \(detail)")
            }
            print("Switch verification failed: \(error), CLI output: \(output)")
        }
    }

    func refreshQuotas() {
        isRefreshing = true
        DispatchQueue.global(qos: .userInitiated).async { [weak self] in
            guard let self = self else { return }
            self.runCLICommand(["quota"])
            DispatchQueue.main.async {
                self.readAccountsFile()
                self.isRefreshing = false
                self.lastRefreshed = Date()
            }
        }
    }

    func removeAccount(id: String) {
        runCLICommand(["remove", id])
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.3) { [weak self] in
            self?.readAccountsFile()
        }
    }

    func addAPIKey(name: String, key: String, rpm: Int = 15, rpd: Int = 1500) {
        runCLICommand(["add-key", "--name", name, "--key", key, "--rpm", "\(rpm)", "--rpd", "\(rpd)"])
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.5) { [weak self] in
            self?.readAccountsFile()
        }
    }

    func loginOAuth(ide: Bool = false, completion: ((Bool, String) -> Void)? = nil) {
        // Run off the main thread: the browser OAuth flow blocks for minutes
        // while the user signs in; blocking main would freeze the UI.
        DispatchQueue.global(qos: .userInitiated).async { [weak self] in
            guard let self = self else { return }
            let output = self.runCLICommand(ide ? ["login", "--ide"] : ["login"])
            DispatchQueue.main.async {
                self.readAccountsFile()
                if output.contains("Successfully added Google account") {
                    self.showToast("Google account added!")
                    completion?(true, "")
                } else {
                    let msg = output.trimmingCharacters(in: .whitespacesAndNewlines)
                    completion?(false, msg.isEmpty ? "Sign-in failed with no output" : msg)
                }
            }
        }
    }

    func importFromAntigravity(completion: ((Bool, String) -> Void)? = nil) {
        DispatchQueue.global(qos: .userInitiated).async { [weak self] in
            guard let self = self else { return }
            let output = self.runCLICommand(["resync"])
            DispatchQueue.main.async {
                self.readAccountsFile()
                if output.contains("Imported live Antigravity session") {
                    self.showToast("Antigravity session imported!")
                    completion?(true, "")
                } else {
                    let msg = output.trimmingCharacters(in: .whitespacesAndNewlines)
                    completion?(false, msg.isEmpty ? "No Antigravity session found" : msg)
                }
            }
        }
    }

    func showToast(_ message: String) {
        DispatchQueue.main.async {
            self.toastMessage = message
        }
        DispatchQueue.main.asyncAfter(deadline: .now() + 3.0) { [weak self] in
            if self?.toastMessage == message {
                self?.toastMessage = nil
            }
        }
    }

    func exportAccount(id: String, destinationURL: URL? = nil) -> (shareCode: String?, filePath: String?) {
        var args = ["export", id]
        if let dest = destinationURL {
            args += ["--file", dest.path]
        }
        let output = runCLICommand(args)
        var code: String? = nil
        var path: String? = nil
        for line in output.components(separatedBy: "\n") {
            let trimmed = line.trimmingCharacters(in: .whitespaces)
            if trimmed.hasPrefix("gswap_") {
                code = trimmed
            } else if trimmed.hasPrefix("📁 Saved config file to:") {
                path = trimmed.replacingOccurrences(of: "📁 Saved config file to:", with: "").trimmingCharacters(in: .whitespaces)
            }
        }
        return (code, path)
    }

    func copyShareCode(for accountId: String) {
        let (code, _) = exportAccount(id: accountId)
        if let code = code {
            NSPasteboard.general.clearContents()
            NSPasteboard.general.setString(code, forType: .string)
            showToast("Share code copied to clipboard!")
        } else {
            showToast("Failed to generate share code")
        }
    }

    func exportToFile(for account: Account) {
        let panel = NSSavePanel()
        panel.title = "Export Gemini Account Config"
        let safeName = account.name
            .replacingOccurrences(of: "@", with: "_")
            .replacingOccurrences(of: " ", with: "_")
            .replacingOccurrences(of: ".", with: "_")
        panel.nameFieldStringValue = "gemini-\(safeName).json"
        panel.allowedContentTypes = [.json]
        if panel.runModal() == .OK, let url = panel.url {
            let (_, path) = exportAccount(id: account.id, destinationURL: url)
            if path != nil {
                showToast("Exported to \(url.lastPathComponent)")
            } else {
                showToast("Failed to save config file")
            }
        }
    }

    func importAccount(source: String) -> (success: Bool, message: String) {
        let trimmed = source.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else {
            return (false, "Input cannot be empty")
        }
        let output = runCLICommand(["import", trimmed])
        if output.contains("imported successfully") {
            readAccountsFile()
            showToast("Account imported successfully!")
            return (true, "Account imported successfully!")
        } else {
            let err = output.trimmingCharacters(in: .whitespacesAndNewlines)
            return (false, err.isEmpty ? "Failed to import account. Please verify file format or share code." : err)
        }
    }

    func importFromFile(completion: ((Bool, String) -> Void)? = nil) {
        let panel = NSOpenPanel()
        panel.title = "Import Gemini Account Config"
        panel.allowedContentTypes = [.json]
        panel.allowsMultipleSelection = false
        if panel.runModal() == .OK, let url = panel.url {
            let result = importAccount(source: url.path)
            completion?(result.success, result.message)
        }
    }

    @discardableResult
    private func runCLICommand(_ arguments: [String]) -> String {
        // Try local build bin or PATH
        let possiblePaths = [
            Bundle.main.resourcePath.map { "\($0)/gemini-swap" },
            Bundle.main.bundlePath + "/Contents/MacOS/gemini-swap",
            "/Users/leuzery/gemini_swap/bin/gemini-swap",
            "/usr/local/bin/gemini-swap",
            "/opt/homebrew/bin/gemini-swap"
        ].compactMap { $0 }

        var executable = "gemini-swap"
        for p in possiblePaths {
            if FileManager.default.isExecutableFile(atPath: p) {
                executable = p
                break
            }
        }

        let process = Process()
        process.executableURL = URL(fileURLWithPath: executable.hasPrefix("/") ? executable : "/usr/bin/env")
        if !executable.hasPrefix("/") {
            process.arguments = [executable] + arguments
        } else {
            process.arguments = arguments
        }

        let pipe = Pipe()
        process.standardOutput = pipe
        process.standardError = pipe

        do {
            try process.run()
            process.waitUntilExit()
            let data = pipe.fileHandleForReading.readDataToEndOfFile()
            return String(data: data, encoding: .utf8) ?? ""
        } catch {
            print("Failed to run CLI command: \(error)")
            return ""
        }
    }

    private func startWatchingFiles() {
        guard FileManager.default.fileExists(atPath: baseDir.path) else { return }

        fileDescriptor = open(baseDir.path, O_EVTONLY)
        guard fileDescriptor >= 0 else { return }

        fileWatcherSource = DispatchSource.makeFileSystemObjectSource(
            fileDescriptor: fileDescriptor,
            eventMask: [.write, .extend, .attrib, .link],
            queue: DispatchQueue.main
        )

        fileWatcherSource?.setEventHandler { [weak self] in
            self?.readAccountsFile()
            self?.readStatsFile()
        }

        fileWatcherSource?.setCancelHandler { [weak self] in
            if let fd = self?.fileDescriptor, fd >= 0 {
                close(fd)
            }
        }

        fileWatcherSource?.resume()
    }

    private func stopWatchingFiles() {
        fileWatcherSource?.cancel()
        fileWatcherSource = nil
    }

    private func startStatsTimer() {
        statsTimer = Timer.scheduledTimer(withTimeInterval: 3.0, repeats: true) { [weak self] _ in
            self?.readStatsFile()
        }
    }
}
