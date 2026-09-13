import SwiftUI

struct MainWindowView: View {
    @ObservedObject var appState: AppState
    var onToggleUpperBar: () -> Void

    @State private var showAddAccountSheet = false
    @State private var addAccountInitialTab = 0

    var body: some View {
        VStack(spacing: 0) {
            // macOS Top Toolbar Header
            HStack(alignment: .center) {
                HStack(spacing: 12) {
                    ZStack {
                        LinearGradient(
                            colors: [Color.blue, Color.purple],
                            startPoint: .topLeading,
                            endPoint: .bottomTrailing
                        )
                        .frame(width: 36, height: 36)
                        .cornerRadius(10)

                        Image(systemName: "sparkles")
                            .font(.system(size: 18, weight: .bold))
                            .foregroundColor(.white)
                    }

                    VStack(alignment: .leading, spacing: 2) {
                        Text("Gemini Swap")
                            .font(.system(size: 17, weight: .bold))
                        Text("\(appState.accounts.count) account\(appState.accounts.count == 1 ? "" : "s") configured")
                            .font(.system(size: 11))
                            .foregroundColor(.secondary)
                    }
                }

                Spacer()

                // Actions
                HStack(spacing: 10) {
                    // Hide to Menu Bar Mode Button ("Upper Part Mode")
                    Button(action: onToggleUpperBar) {
                        HStack(spacing: 6) {
                            Image(systemName: "menubar.arrow.up.rectangle")
                            Text("Upper Bar Mode")
                        }
                    }
                    .buttonStyle(.bordered)
                    .help("Hide app into the top macOS Menu Bar")

                    // Refresh Button
                    Button(action: {
                        appState.refreshQuotas()
                    }) {
                        HStack(spacing: 4) {
                            Image(systemName: "arrow.clockwise")
                                .rotationEffect(.degrees(appState.isRefreshing ? 360 : 0))
                                .animation(appState.isRefreshing ? .linear(duration: 1).repeatForever(autoreverses: false) : .default, value: appState.isRefreshing)
                            Text("Refresh Quotas")
                        }
                    }
                    .buttonStyle(.bordered)
                    .disabled(appState.isRefreshing)

                    // Import Button
                    Button(action: {
                        addAccountInitialTab = 2
                        showAddAccountSheet = true
                    }) {
                        HStack(spacing: 4) {
                            Image(systemName: "square.and.arrow.down")
                            Text("Import...")
                        }
                    }
                    .buttonStyle(.bordered)
                    .help("Import account config from a friend")

                    // Add Account Button
                    Button(action: {
                        addAccountInitialTab = 0
                        showAddAccountSheet = true
                    }) {
                        HStack(spacing: 4) {
                            Image(systemName: "plus")
                            Text("Add Account")
                        }
                    }
                    .buttonStyle(.borderedProminent)
                    .tint(.blue)
                }
            }
            .padding(.horizontal, 20)
            .padding(.vertical, 16)
            .background(Color(NSColor.windowBackgroundColor))
            
            Divider()

            // Main Content Area
            ScrollView {
                VStack(spacing: 18) {
                    if let toast = appState.toastMessage {
                        HStack(spacing: 8) {
                            Image(systemName: "checkmark.circle.fill")
                                .foregroundColor(.green)
                            Text(toast)
                                .font(.system(size: 13, weight: .medium))
                            Spacer()
                        }
                        .padding(12)
                        .background(Color.green.opacity(0.15))
                        .cornerRadius(8)
                    }

                    // Accounts Section
                    VStack(alignment: .leading, spacing: 12) {
                        Text("Configured Accounts")
                            .font(.system(size: 14, weight: .bold))
                            .foregroundColor(.secondary)

                        if appState.accounts.isEmpty {
                            VStack(spacing: 12) {
                                Image(systemName: "person.crop.circle.badge.plus")
                                    .font(.system(size: 40))
                                    .foregroundColor(.secondary)
                                Text("No Gemini accounts added yet")
                                    .font(.headline)
                                Text("Add a Google OAuth account or an AI Studio API key to begin monitoring usage and swapping effortlessly.")
                                    .font(.subheadline)
                                    .foregroundColor(.secondary)
                                    .multilineTextAlignment(.center)
                                    .frame(maxWidth: 400)
                                Button("Add Your First Account") {
                                    addAccountInitialTab = 0
                                    showAddAccountSheet = true
                                }
                                .buttonStyle(.borderedProminent)
                                .tint(.blue)
                            }
                            .frame(maxWidth: .infinity)
                            .padding(.vertical, 40)
                        } else {
                            ForEach(appState.accounts) { acc in
                                AccountCardView(
                                    account: acc,
                                    isActive: acc.id == appState.activeAccountID,
                                    onSwitch: {
                                        appState.switchAccount(to: acc.id)
                                    },
                                    onRemove: {
                                        appState.removeAccount(id: acc.id)
                                    },
                                    onExportFile: {
                                        appState.exportToFile(for: acc)
                                    },
                                    onCopyShareCode: {
                                        appState.copyShareCode(for: acc.id)
                                    }
                                )
                            }
                        }
                    }

                    // Proxy & Codex Section
                    ProxyLogsView(stats: appState.proxyStats)
                }
                .padding(20)
            }

            Divider()

            // Bottom Footer / Status
            HStack {
                if let active = appState.activeAccount {
                    HStack(spacing: 6) {
                        Circle()
                            .fill(Color.green)
                            .frame(width: 7, height: 7)
                        Text("Active: \(active.name)")
                            .font(.system(size: 11, weight: .medium))
                    }
                } else {
                    Text("No active account")
                        .font(.system(size: 11))
                        .foregroundColor(.secondary)
                }

                Spacer()

                if let lastRef = appState.lastRefreshed {
                    Text("Updated: \(lastRef.formatted(date: .omitted, time: .standard))")
                        .font(.system(size: 11))
                        .foregroundColor(.secondary)
                }
            }
            .padding(.horizontal, 20)
            .padding(.vertical, 8)
            .background(Color(NSColor.windowBackgroundColor))
        }
        .frame(minWidth: 680, minHeight: 520)
        .sheet(isPresented: $showAddAccountSheet) {
            AddAccountSheet(appState: appState, initialTab: addAccountInitialTab)
        }
    }
}
