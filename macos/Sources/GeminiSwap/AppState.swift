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

    private var fileWatcherSource: DispatchSourceFileSystemObject?
    private var fileDescriptor: Int32 = -1
    private var statsTimer: Timer?

    static let shared = AppState()

    var activeAccount: Account? {
        accounts.first(where: { $0.id == activeAccountID }) ?? accounts.first
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
        guard let target = accounts.first(where: { $0.id == id }) else { return }

        // Update local state
        activeAccountID = id
        
        // Run CLI switch command to update all stores, Antigravity credentials, and ~/.gemini/
        runCLICommand(["switch", id])

        // Ensure CLI and Antigravity tokens are synced
        syncToGeminiCLI(account: target)
        syncToAntigravity(account: target)

        // Reload updated accounts data and distinct quotas
        readAccountsFile()
        showToast("Switched to \(target.name) (Antigravity & CLI synced)")
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

    func loginOAuth() {
        runCLICommand(["login"])
        DispatchQueue.main.asyncAfter(deadline: .now() + 1.0) { [weak self] in
            self?.readAccountsFile()
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

    private func syncToGeminiCLI(account: Account) {
        let geminiDir = FileManager.default.homeDirectoryForCurrentUser.appendingPathComponent(".gemini")
        try? FileManager.default.createDirectory(at: geminiDir, withIntermediateDirectories: true)

        if account.type == .oauth, let oauth = account.oauth {
            let credsPath = geminiDir.appendingPathComponent("oauth_creds.json")
            let accountsPath = geminiDir.appendingPathComponent("google_accounts.json")

            let credsDict: [String: Any] = [
                "access_token": oauth.accessToken,
                "refresh_token": oauth.refreshToken ?? "",
                "scope": oauth.scope ?? "",
                "token_type": oauth.tokenType ?? "Bearer",
                "id_token": oauth.idToken ?? "",
                "expiry_date": oauth.expiryDate ?? 0
            ]

            if let credsData = try? JSONSerialization.data(withJSONObject: credsDict, options: .prettyPrinted) {
                try? credsData.write(to: credsPath)
            }

            if let email = account.email {
                let accDict: [String: Any] = [
                    "active": email,
                    "old": []
                ]
                if let accData = try? JSONSerialization.data(withJSONObject: accDict, options: .prettyPrinted) {
                    try? accData.write(to: accountsPath)
                }
            }
        }
    }

    private func syncToAntigravity(account: Account) {
        guard account.type == .oauth, let oauth = account.oauth else { return }

        let geminiDir = FileManager.default.homeDirectoryForCurrentUser.appendingPathComponent(".gemini")
        try? FileManager.default.createDirectory(at: geminiDir, withIntermediateDirectories: true)

        let expiryDate: Date
        if let exp = oauth.expiryDate, exp > 0 {
            expiryDate = Date(timeIntervalSince1970: Double(exp) / 1000.0)
        } else {
            expiryDate = Date().addingTimeInterval(3600)
        }
        let expiryStr = ISO8601DateFormatter().string(from: expiryDate)

        let standaloneDict: [String: Any] = [
            "token": [
                "access_token": oauth.accessToken,
                "token_type": oauth.tokenType ?? "Bearer",
                "refresh_token": oauth.refreshToken ?? "",
                "expiry": expiryStr
            ],
            "auth_method": "consumer"
        ]

        guard let jsonData = try? JSONSerialization.data(withJSONObject: standaloneDict, options: []) else { return }

        // 1. Write ~/.gemini/jetski-standalone-oauth-token
        let tokenURL = geminiDir.appendingPathComponent("jetski-standalone-oauth-token")
        try? jsonData.write(to: tokenURL)
        try? FileManager.default.setAttributes([.posixPermissions: 0o600], ofItemAtPath: tokenURL.path)

        // 2. Update macOS Keychain
        let b64 = jsonData.base64EncodedString()
        let keychainVal = "go-keyring-base64:" + b64
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/usr/bin/security")
        process.arguments = ["add-generic-password", "-U", "-s", "gemini", "-a", "antigravity", "-w", keychainVal]
        try? process.run()
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
