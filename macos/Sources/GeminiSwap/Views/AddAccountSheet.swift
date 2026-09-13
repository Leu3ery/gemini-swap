import SwiftUI

struct AddAccountSheet: View {
    @Environment(\.dismiss) var dismiss
    @ObservedObject var appState: AppState

    @State private var selectedTab: Int
    @State private var keyName: String = ""
    @State private var apiKey: String = ""
    @State private var rpmLimit: String = "15"
    @State private var rpdLimit: String = "1500"
    @State private var shareCode: String = ""
    @State private var errorMessage: String? = nil
    @State private var isSubmitting: Bool = false

    init(appState: AppState, initialTab: Int = 0) {
        self.appState = appState
        self._selectedTab = State(initialValue: initialTab)
    }

    var body: some View {
        VStack(spacing: 20) {
            // Header
            HStack {
                Text(selectedTab == 2 ? "Import Account" : "Add Gemini Account")
                    .font(.headline)
                Spacer()
                Button(action: { dismiss() }) {
                    Image(systemName: "xmark.circle.fill")
                        .foregroundColor(.secondary)
                }
                .buttonStyle(.plain)
            }

            Picker("", selection: $selectedTab) {
                Text("Google OAuth").tag(0)
                Text("API Key").tag(1)
                Text("Import from Friend").tag(2)
            }
            .pickerStyle(.segmented)

            if selectedTab == 0 {
                // OAuth Flow
                VStack(spacing: 16) {
                    Image(systemName: "person.badge.key.fill")
                        .font(.system(size: 44))
                        .foregroundColor(.blue)
                        .padding(.top, 10)

                    Text("Sign in with your Google account")
                        .font(.system(size: 14, weight: .medium))

                    Text("Enables full access to Gemini 2.5/3.0 models via Google Gemini Code Assist with 1,000 free requests/day.")
                        .font(.system(size: 12))
                        .foregroundColor(.secondary)
                        .multilineTextAlignment(.center)
                        .padding(.horizontal)

                    Button(action: {
                        isSubmitting = true
                        appState.loginOAuth()
                        DispatchQueue.main.asyncAfter(deadline: .now() + 1.0) {
                            dismiss()
                        }
                    }) {
                        HStack {
                            Image(systemName: "globe")
                            Text("Sign in with Google in Browser")
                        }
                        .frame(maxWidth: .infinity)
                    }
                    .buttonStyle(.borderedProminent)
                    .tint(.blue)
                    .controlSize(.large)
                    .padding(.top, 10)
                }
                .padding(.vertical, 10)
            } else if selectedTab == 1 {
                // API Key Flow
                VStack(alignment: .leading, spacing: 12) {
                    VStack(alignment: .leading, spacing: 4) {
                        Text("Account Label")
                            .font(.system(size: 12, weight: .medium))
                        TextField("e.g. AI Studio Secondary Key", text: $keyName)
                            .textFieldStyle(.roundedBorder)
                    }

                    VStack(alignment: .leading, spacing: 4) {
                        Text("Gemini API Key")
                            .font(.system(size: 12, weight: .medium))
                        SecureField("AIzaSy...", text: $apiKey)
                            .textFieldStyle(.roundedBorder)
                    }

                    HStack(spacing: 16) {
                        VStack(alignment: .leading, spacing: 4) {
                            Text("RPM Limit")
                                .font(.system(size: 12, weight: .medium))
                            TextField("15", text: $rpmLimit)
                                .textFieldStyle(.roundedBorder)
                        }

                        VStack(alignment: .leading, spacing: 4) {
                            Text("Daily Limit (RPD)")
                                .font(.system(size: 12, weight: .medium))
                            TextField("1500", text: $rpdLimit)
                                .textFieldStyle(.roundedBorder)
                        }
                    }

                    Button(action: {
                        guard !apiKey.isEmpty else { return }
                        let name = keyName.isEmpty ? "API Key Account" : keyName
                        let rpm = Int(rpmLimit) ?? 15
                        let rpd = Int(rpdLimit) ?? 1500
                        appState.addAPIKey(name: name, key: apiKey, rpm: rpm, rpd: rpd)
                        dismiss()
                    }) {
                        Text("Add Account")
                            .frame(maxWidth: .infinity)
                    }
                    .buttonStyle(.borderedProminent)
                    .tint(.blue)
                    .disabled(apiKey.isEmpty)
                    .padding(.top, 8)
                }
            } else {
                // Import Flow (from File or Share Code)
                VStack(spacing: 14) {
                    Text("Load an account configuration shared by a friend.")
                        .font(.system(size: 12))
                        .foregroundColor(.secondary)
                        .multilineTextAlignment(.center)

                    Button(action: {
                        appState.importFromFile { success, msg in
                            if success {
                                dismiss()
                            } else {
                                errorMessage = msg
                            }
                        }
                    }) {
                        HStack(spacing: 6) {
                            Image(systemName: "doc.badge.plus")
                            Text("Choose Config File (.json)...")
                        }
                        .frame(maxWidth: .infinity)
                    }
                    .buttonStyle(.bordered)
                    .controlSize(.large)

                    HStack {
                        Rectangle()
                            .fill(Color.secondary.opacity(0.2))
                            .frame(height: 1)
                        Text("OR PASTE SHARE CODE")
                            .font(.system(size: 10, weight: .semibold))
                            .foregroundColor(.secondary)
                        Rectangle()
                            .fill(Color.secondary.opacity(0.2))
                            .frame(height: 1)
                    }
                    .padding(.vertical, 4)

                    VStack(alignment: .leading, spacing: 6) {
                        Text("Share Code (starts with gswap_)")
                            .font(.system(size: 11, weight: .medium))

                        TextField("Paste code here...", text: $shareCode)
                            .textFieldStyle(.roundedBorder)
                            .font(.system(size: 11, design: .monospaced))
                    }

                    if let err = errorMessage {
                        Text(err)
                            .font(.system(size: 11))
                            .foregroundColor(.red)
                            .multilineTextAlignment(.center)
                    }

                    Button(action: {
                        let (success, msg) = appState.importAccount(source: shareCode)
                        if success {
                            dismiss()
                        } else {
                            errorMessage = msg
                        }
                    }) {
                        HStack {
                            Image(systemName: "arrow.down.circle")
                            Text("Import Account")
                        }
                        .frame(maxWidth: .infinity)
                    }
                    .buttonStyle(.borderedProminent)
                    .tint(.blue)
                    .disabled(shareCode.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
                }
                .padding(.top, 4)
            }
        }
        .padding(24)
        .frame(width: 440)
    }
}
