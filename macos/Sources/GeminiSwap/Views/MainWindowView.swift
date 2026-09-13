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
                Text("Gemini Swap")
                    .font(.system(size: 15, weight: .semibold))

                Spacer()

                // Actions
                HStack(spacing: 8) {
                    Button(action: onToggleUpperBar) {
                        HStack(spacing: 4) {
                            Image(systemName: "menubar.arrow.up.rectangle")
                            Text("Menu Bar")
                        }
                    }
                    .buttonStyle(.bordered)
                    .help("Hide app into the macOS Menu Bar")

                    Button(action: {
                        appState.refreshQuotas()
                    }) {
                        Image(systemName: "arrow.clockwise")
                            .rotationEffect(.degrees(appState.isRefreshing ? 360 : 0))
                            .animation(appState.isRefreshing ? .linear(duration: 1).repeatForever(autoreverses: false) : .default, value: appState.isRefreshing)
                    }
                    .buttonStyle(.bordered)
                    .disabled(appState.isRefreshing)
                    .help("Refresh Quotas")

                    Button(action: {
                        addAccountInitialTab = 2
                        showAddAccountSheet = true
                    }) {
                        HStack(spacing: 4) {
                            Image(systemName: "square.and.arrow.down")
                            Text("Import")
                        }
                    }
                    .buttonStyle(.bordered)
                    .help("Import account config from a friend")

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
                }
            }
            .padding(.horizontal, 20)
            .padding(.vertical, 12)
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

                        if let switching = appState.switchingAccount,
                           let status = appState.switchingStatusText {
                            HStack(spacing: 8) {
                                ProgressView()
                                    .controlSize(.small)
                                TimelineView(.periodic(from: appState.switchStartedAt ?? .now, by: 1)) { context in
                                    let elapsed = max(0, Int(context.date.timeIntervalSince(appState.switchStartedAt ?? context.date)))
                                    Text("\(status) \(switching.name) · \(elapsed)s")
                                        .font(.system(size: 12, weight: .medium))
                                        .foregroundColor(.secondary)
                                }
                            }
                            .accessibilityElement(children: .combine)
                        }

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
                                    switchingText: appState.switchingAccountID == acc.id ? appState.switchingStatusText : nil,
                                    switchStartedAt: appState.switchingAccountID == acc.id ? appState.switchStartedAt : nil,
                                    switchingDisabled: appState.isSwitchingAccount,
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
                            .frame(width: 6, height: 6)
                        Text("Active: \(active.name)")
                            .font(.system(size: 11))
                            .foregroundColor(.secondary)
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
        .frame(minWidth: 580, minHeight: 380)
        .sheet(isPresented: $showAddAccountSheet) {
            AddAccountSheet(appState: appState, initialTab: addAccountInitialTab)
        }
    }
}
