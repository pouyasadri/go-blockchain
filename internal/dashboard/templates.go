package dashboard

import (
	"fmt"
	"html/template"
	"io"
	"strings"
	"time"

	"github.com/pouyasadri/go-blockchain/internal/indexer"
)

const indexHTML = `<!DOCTYPE html>
<html lang="en" class="dark">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>AI Ledger & Autonomous Micro-Payment Engine</title>
    <!-- Premium Fonts -->
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&family=JetBrains+Mono:ital,wght@0,400;0,500;0,600;1,400&family=Outfit:wght@500;600;700;800&display=swap" rel="stylesheet">
    <!-- Tailwind CSS -->
    <script src="https://cdn.tailwindcss.com"></script>
    <!-- HTMX & HTMX SSE extension -->
    <script src="https://unpkg.com/htmx.org@1.9.10"></script>
    <script src="https://unpkg.com/htmx.org@1.9.10/dist/ext/sse.js"></script>
    <!-- Alpine.js -->
    <script defer src="https://unpkg.com/alpinejs@3.x.x/dist/cdn.min.js"></script>
    <script>
        tailwind.config = {
            darkMode: 'class',
            theme: {
                extend: {
                    fontFamily: {
                        sans: ['Inter', 'sans-serif'],
                        outfit: ['Outfit', 'sans-serif'],
                        mono: ['JetBrains Mono', 'monospace'],
                    },
                    colors: {
                        darkBg: '#080D1A',
                        panelBg: 'rgba(15, 23, 42, 0.75)',
                        panelBorder: 'rgba(255, 255, 255, 0.08)',
                    }
                }
            }
        }
    </script>
    <style>
        body {
            background-color: #080D1A;
            background-image: 
                radial-gradient(circle at 15% 15%, rgba(99, 102, 241, 0.15) 0%, transparent 40%),
                radial-gradient(circle at 85% 20%, rgba(168, 85, 247, 0.12) 0%, transparent 45%),
                radial-gradient(circle at 50% 85%, rgba(6, 182, 212, 0.1) 0%, transparent 50%);
            background-attachment: fixed;
            color: #F8FAFC;
            min-height: 100vh;
        }
        .glass-card {
            background: rgba(15, 23, 42, 0.72);
            backdrop-filter: blur(20px);
            -webkit-backdrop-filter: blur(20px);
            border: 1px solid rgba(255, 255, 255, 0.09);
            box-shadow: 0 20px 40px -15px rgba(0, 0, 0, 0.6), inset 0 1px 0 rgba(255, 255, 255, 0.12);
            transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
        }
        .glass-card:hover {
            border-color: rgba(99, 102, 241, 0.35);
            box-shadow: 0 25px 50px -12px rgba(99, 102, 241, 0.18), inset 0 1px 0 rgba(255, 255, 255, 0.2);
        }
        .glow-icon {
            filter: drop-shadow(0 0 8px currentColor);
        }
        /* Custom Scrollbar */
        ::-webkit-scrollbar {
            width: 6px;
            height: 6px;
        }
        ::-webkit-scrollbar-track {
            background: rgba(8, 13, 26, 0.9);
        }
        ::-webkit-scrollbar-thumb {
            background: rgba(255, 255, 255, 0.15);
            border-radius: 9999px;
        }
        ::-webkit-scrollbar-thumb:hover {
            background: rgba(99, 102, 241, 0.5);
        }
    </style>
</head>
<body class="font-sans antialiased p-4 md:p-8" hx-ext="sse" sse-connect="/events" x-data="{ activeTab: 'overview', toast: '', showModal: false, modalService: null, copyToClipboard(text) { navigator.clipboard.writeText(text); this.toast = 'Copied to clipboard!'; setTimeout(() => this.toast = '', 2500); } }">
    
    <!-- Toast Notification -->
    <div x-show="toast" x-transition:enter="transition ease-out duration-300 transform" x-transition:enter-start="opacity-0 translate-y-3 scale-95" x-transition:enter-end="opacity-100 translate-y-0 scale-100" x-transition:leave="transition ease-in duration-200" class="fixed bottom-6 right-6 z-50 px-4 py-3 rounded-2xl bg-slate-900/95 border border-cyan-500/40 text-cyan-300 text-xs font-semibold shadow-2xl flex items-center gap-3 backdrop-blur-xl" style="display: none;">
        <div class="p-1 rounded-lg bg-cyan-500/20 text-cyan-400">
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/></svg>
        </div>
        <span x-text="toast"></span>
    </div>

    <!-- Main Container -->
    <div class="max-w-7xl mx-auto space-y-8">
        
        <!-- Header Bar -->
        <header class="flex flex-col lg:flex-row lg:items-center justify-between gap-6 pb-6 border-b border-slate-800/80">
            <div class="flex items-center gap-4">
                <div class="relative flex items-center justify-center w-12 h-12 rounded-2xl bg-gradient-to-br from-indigo-500 via-purple-500 to-cyan-400 p-0.5 shadow-xl shadow-indigo-500/20">
                    <div class="w-full h-full bg-slate-950 rounded-[14px] flex items-center justify-center">
                        <svg class="w-6 h-6 text-cyan-400 glow-icon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19.428 15.428a2 2 0 00-1.022-.547l-2.387-.477a6 6 0 00-3.86.517l-.318.158a6 6 0 01-3.86.517L5.594 15.12a2 2 0 00-1.678.85l-.756 1.135a1 1 0 00.178 1.34l3.183 2.728a2 2 0 001.306.477h8.346a2 2 0 001.306-.477l3.183-2.728a1 1 0 00.178-1.34l-.756-1.135z"/>
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 3v9m0 0l-3-3m3 3l3-3"/>
                        </svg>
                    </div>
                </div>
                <div>
                    <h1 class="font-outfit text-2xl md:text-3xl font-extrabold tracking-tight bg-gradient-to-r from-white via-slate-100 to-indigo-300 bg-clip-text text-transparent">
                        AI Micro-payment Settlement Engine
                    </h1>
                    <p class="text-xs md:text-sm text-slate-400 flex items-center gap-2 mt-0.5 font-medium">
                        <span class="inline-block w-2 h-2 rounded-full bg-emerald-400 shadow-sm shadow-emerald-400"></span>
                        Autonomous Agent Coordination Mesh & Financial Firewall
                    </p>
                </div>
            </div>

            <!-- Network Badges -->
            <div class="flex items-center gap-3">
                <div class="flex items-center gap-2 px-3.5 py-1.5 rounded-full bg-emerald-500/10 border border-emerald-500/30 text-emerald-400 text-xs font-semibold tracking-wide">
                    <span class="relative flex h-2 w-2">
                        <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
                        <span class="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
                    </span>
                    gRPC :50051 ONLINE
                </div>
                <div class="flex items-center gap-2 px-3.5 py-1.5 rounded-full bg-cyan-500/10 border border-cyan-500/30 text-cyan-300 text-xs font-semibold tracking-wide">
                    <span class="relative flex h-2 w-2">
                        <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-cyan-400 opacity-75"></span>
                        <span class="relative inline-flex rounded-full h-2 w-2 bg-cyan-400"></span>
                    </span>
                    SSE CONNECTED
                </div>
            </div>
        </header>

        <!-- Navigation Tabs -->
        <nav class="flex items-center gap-2 overflow-x-auto pb-2 border-b border-slate-800/60 text-sm font-medium">
            <button @click="activeTab = 'overview'" :class="activeTab === 'overview' ? 'bg-indigo-600/20 text-indigo-300 border-indigo-500/50 shadow-lg shadow-indigo-500/10' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/40 border-transparent'" class="flex items-center gap-2 px-4 py-2 rounded-xl border transition-all">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2V6zM14 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2V6zM4 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2v-2zM14 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2v-2z"/></svg>
                Overview
            </button>
            <button @click="activeTab = 'escrows'" :class="activeTab === 'escrows' ? 'bg-indigo-600/20 text-indigo-300 border-indigo-500/50 shadow-lg shadow-indigo-500/10' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/40 border-transparent'" class="flex items-center gap-2 px-4 py-2 rounded-xl border transition-all">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"/></svg>
                HTLC / ZKCP Escrows
            </button>
            <button @click="activeTab = 'marketplace'" :class="activeTab === 'marketplace' ? 'bg-indigo-600/20 text-indigo-300 border-indigo-500/50 shadow-lg shadow-indigo-500/10' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/40 border-transparent'" class="flex items-center gap-2 px-4 py-2 rounded-xl border transition-all">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10"/></svg>
                Agent Marketplace
            </button>
            <button @click="activeTab = 'firewall'" :class="activeTab === 'firewall' ? 'bg-indigo-600/20 text-indigo-300 border-indigo-500/50 shadow-lg shadow-indigo-500/10' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/40 border-transparent'" class="flex items-center gap-2 px-4 py-2 rounded-xl border transition-all">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"/></svg>
                Financial Firewall
            </button>
            <button @click="activeTab = 'blocks'" :class="activeTab === 'blocks' ? 'bg-indigo-600/20 text-indigo-300 border-indigo-500/50 shadow-lg shadow-indigo-500/10' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/40 border-transparent'" class="flex items-center gap-2 px-4 py-2 rounded-xl border transition-all">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"/></svg>
                Block Stream
            </button>
        </nav>

        <!-- Stats Top Telemetry Grid -->
        <div id="metrics-bar" hx-get="/partials/metrics" hx-trigger="load, sse:metrics" class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-5">
            <!-- Rendered dynamically -->
        </div>

        <!-- Main Content Area -->
        <div class="grid grid-cols-1 lg:grid-cols-3 gap-8">
            
            <!-- Left Column: Active Escrows & Peer Marketplace (2 cols) -->
            <div class="lg:col-span-2 space-y-8">
                
                <!-- HTLC Escrows Section -->
                <section x-show="activeTab === 'overview' || activeTab === 'escrows'" class="glass-card rounded-3xl p-6 space-y-5">
                    <div class="flex items-center justify-between">
                        <div class="flex items-center gap-3">
                            <div class="p-2.5 rounded-2xl bg-gradient-to-br from-indigo-500/20 to-purple-500/20 text-indigo-400 border border-indigo-500/30">
                                <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"/></svg>
                            </div>
                            <div>
                                <h2 class="font-outfit text-lg font-bold text-slate-100">Active HTLC & ZKCP Escrows</h2>
                                <p class="text-xs text-slate-400">Cryptographically locked micro-settlement contracts</p>
                            </div>
                        </div>
                        <span class="flex items-center gap-1.5 text-xs text-slate-400 font-mono bg-slate-900/80 px-3 py-1 rounded-xl border border-slate-800">
                            <span class="w-2 h-2 rounded-full bg-cyan-400 animate-ping"></span>
                            Live Ledger
                        </span>
                    </div>

                    <div id="escrow-list" hx-get="/partials/escrows" hx-trigger="load, sse:escrows" class="space-y-3.5">
                        <div class="p-8 text-center text-slate-500 text-sm">Loading escrows...</div>
                    </div>
                </section>

                <!-- AI Marketplace Catalog Section -->
                <section x-show="activeTab === 'overview' || activeTab === 'marketplace'" class="glass-card rounded-3xl p-6 space-y-5">
                    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
                        <div class="flex items-center gap-3">
                            <div class="p-2.5 rounded-2xl bg-gradient-to-br from-emerald-500/20 to-teal-500/20 text-emerald-400 border border-emerald-500/30">
                                <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 13.255A23.931 23.931 0 0112 15c-3.183 0-6.22-.62-9-1.745M16 6V4a2 2 0 00-2-2h-4a2 2 0 00-2 2v2m4 6h.01M5 20h14a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"/></svg>
                            </div>
                            <div>
                                <h2 class="font-outfit text-lg font-bold text-slate-100">AI Peer Marketplace Catalog</h2>
                                <p class="text-xs text-slate-400">Off-chain & On-chain agent services requiring micro-settlement</p>
                            </div>
                        </div>
                    </div>

                    <div id="service-list" hx-get="/partials/services" hx-trigger="load" class="grid grid-cols-1 md:grid-cols-2 gap-4">
                        <div class="p-8 text-center text-slate-500 text-sm col-span-2">Loading marketplace services...</div>
                    </div>
                </section>
            </div>

            <!-- Right Column: Financial Firewall & Block Stream (1 col) -->
            <div class="space-y-8">
                
                <!-- Financial Firewall Meter -->
                <section x-show="activeTab === 'overview' || activeTab === 'firewall'" class="glass-card rounded-3xl p-6 space-y-5 border-l-4 border-indigo-500">
                    <div class="flex items-center gap-3">
                        <div class="p-2.5 rounded-2xl bg-indigo-500/20 text-indigo-400 border border-indigo-500/30">
                            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"/></svg>
                        </div>
                        <div>
                            <h2 class="font-outfit text-lg font-bold text-slate-100">Financial Firewall</h2>
                            <p class="text-xs text-slate-400">Session Expenditure Guard</p>
                        </div>
                    </div>

                    <div id="firewall-meter" hx-get="/partials/firewall" hx-trigger="load, sse:firewall" class="space-y-4">
                        <div class="p-4 text-center text-slate-500 text-sm">Loading budget telemetry...</div>
                    </div>
                </section>

                <!-- Recent Blocks Feed -->
                <section x-show="activeTab === 'overview' || activeTab === 'blocks'" class="glass-card rounded-3xl p-6 space-y-5">
                    <div class="flex items-center justify-between">
                        <div class="flex items-center gap-3">
                            <div class="p-2.5 rounded-2xl bg-amber-500/20 text-amber-400 border border-amber-500/30">
                                <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"/></svg>
                            </div>
                            <div>
                                <h2 class="font-outfit text-lg font-bold text-slate-100">Block Stream Feed</h2>
                                <p class="text-xs text-slate-400">Real-time ledger block feed</p>
                            </div>
                        </div>
                    </div>

                    <div id="block-list" hx-get="/partials/blocks" hx-trigger="load, sse:blocks" class="space-y-3">
                        <div class="p-4 text-center text-slate-500 text-sm">Listening for blocks...</div>
                    </div>
                </section>
            </div>
        </div>
    </div>
</body>
</html>`

// TemplateRenderer handles generating HTML partials
type TemplateRenderer struct {
	tpl *template.Template
}

// NewTemplateRenderer compiles dashboard templates
func NewTemplateRenderer() (*TemplateRenderer, error) {
	t, err := template.New("index").Parse(indexHTML)
	if err != nil {
		return nil, fmt.Errorf("failed to parse index html template: %w", err)
	}
	return &TemplateRenderer{tpl: t}, nil
}

// RenderIndex renders the full single page application
func (tr *TemplateRenderer) RenderIndex(w io.Writer) error {
	return tr.tpl.Execute(w, nil)
}

// RenderMetricsPartial renders metrics header cards
func RenderMetricsPartial(w io.Writer, m indexer.Metrics) {
	html := fmt.Sprintf(`
	<div class="glass-card p-5 rounded-2xl border-l-4 border-indigo-500 flex items-center justify-between">
		<div class="space-y-1">
			<span class="text-slate-400 text-xs font-semibold uppercase tracking-wider block">Blocks Indexed</span>
			<div class="text-3xl font-extrabold font-outfit text-white tracking-tight">%d</div>
			<span class="text-[11px] text-emerald-400 flex items-center gap-1 font-medium">
				<span class="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse"></span>
				Verified Chain Height
			</span>
		</div>
		<div class="p-3 rounded-2xl bg-indigo-500/10 text-indigo-400 border border-indigo-500/20 shadow-lg shadow-indigo-500/10">
			<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10"/></svg>
		</div>
	</div>

	<div class="glass-card p-5 rounded-2xl border-l-4 border-cyan-500 flex items-center justify-between">
		<div class="space-y-1">
			<span class="text-slate-400 text-xs font-semibold uppercase tracking-wider block">Total Transactions</span>
			<div class="text-3xl font-extrabold font-outfit text-white tracking-tight">%d</div>
			<span class="text-[11px] text-cyan-400 font-mono font-medium">100%% Validated UTXOs</span>
		</div>
		<div class="p-3 rounded-2xl bg-cyan-500/10 text-cyan-400 border border-cyan-500/20 shadow-lg shadow-cyan-500/10">
			<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4"/></svg>
		</div>
	</div>

	<div class="glass-card p-5 rounded-2xl border-l-4 border-purple-500 flex items-center justify-between">
		<div class="space-y-1">
			<span class="text-slate-400 text-xs font-semibold uppercase tracking-wider block">Active Escrows</span>
			<div class="text-3xl font-extrabold font-outfit text-white tracking-tight">%d</div>
			<span class="text-[11px] text-purple-400 font-medium">HTLC & ZKCP Contracts</span>
		</div>
		<div class="p-3 rounded-2xl bg-purple-500/10 text-purple-400 border border-purple-500/20 shadow-lg shadow-purple-500/10">
			<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"/></svg>
		</div>
	</div>

	<div class="glass-card p-5 rounded-2xl border-l-4 border-amber-500 flex items-center justify-between">
		<div class="space-y-1">
			<span class="text-slate-400 text-xs font-semibold uppercase tracking-wider block">Settlement Volume</span>
			<div class="text-3xl font-extrabold font-outfit text-white tracking-tight">%d <span class="text-xs font-mono text-amber-400/90 font-semibold">μc</span></div>
			<span class="text-[11px] text-amber-400 font-medium">Micro-Payment Ledger</span>
		</div>
		<div class="p-3 rounded-2xl bg-amber-500/10 text-amber-400 border border-amber-500/20 shadow-lg shadow-amber-500/10">
			<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"/></svg>
		</div>
	</div>
	`, m.TotalBlocksIndexed, m.TotalTxCount, m.TotalEscrowsCount, m.TotalVolume)
	_, _ = fmt.Fprint(w, html)
}

// RenderEscrowsPartial renders active escrows list
func RenderEscrowsPartial(w io.Writer, escrows []*indexer.IndexedEscrow) {
	if len(escrows) == 0 {
		_, _ = fmt.Fprint(w, `<div class="p-8 text-center text-slate-500 text-sm rounded-2xl bg-slate-900/40 border border-slate-800/80">No active HTLC or ZKCP escrows currently pending settlement.</div>`)
		return
	}

	var sb strings.Builder
	for _, e := range escrows {
		statusBadge := "bg-amber-500/10 text-amber-300 border-amber-500/30"
		switch e.Status {
		case indexer.EscrowStatusClaimed:
			statusBadge = "bg-emerald-500/10 text-emerald-300 border-emerald-500/30"
		case indexer.EscrowStatusRefunded:
			statusBadge = "bg-rose-500/10 text-rose-300 border-rose-500/30"
		case indexer.EscrowStatusExpired:
			statusBadge = "bg-slate-500/10 text-slate-300 border-slate-500/30"
		}

		fmt.Fprintf(&sb, `
		<div class="p-4 rounded-2xl bg-slate-900/60 border border-slate-800/80 hover:border-indigo-500/40 transition-all space-y-3">
			<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
				<div class="flex items-center gap-2 flex-wrap">
					<button @click="copyToClipboard('%s')" title="Click to copy TX ID" class="font-mono text-xs text-indigo-400 bg-indigo-500/10 px-2.5 py-1 rounded-lg border border-indigo-500/20 font-semibold hover:bg-indigo-500/20 transition-all">TX %s...</button>
					<span class="px-2.5 py-1 rounded-lg text-xs font-semibold border %s">%s</span>
					<span class="px-2 py-0.5 rounded-md text-[10px] font-semibold bg-purple-500/10 text-purple-300 border border-purple-500/30 flex items-center gap-1 font-mono">
						<svg class="w-3 h-3 text-purple-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"/></svg>
						ZKCP Proof Valid
					</span>
				</div>
				<div class="text-right">
					<span class="text-base font-extrabold font-outfit text-white">%d μ-cents</span>
				</div>
			</div>
			
			<div class="grid grid-cols-1 sm:grid-cols-2 gap-2 text-xs text-slate-400 pt-2 border-t border-slate-800/50">
				<div>
					<span class="text-slate-500">Buyer:</span> 
					<code @click="copyToClipboard('%s')" title="Click to copy address" class="font-mono text-slate-300 bg-slate-950 px-1.5 py-0.5 rounded border border-slate-800 hover:border-slate-600 cursor-pointer">%s...</code>
				</div>
				<div class="sm:text-right">
					<span class="text-slate-500">Seller:</span> 
					<code @click="copyToClipboard('%s')" title="Click to copy address" class="font-mono text-slate-300 bg-slate-950 px-1.5 py-0.5 rounded border border-slate-800 hover:border-slate-600 cursor-pointer">%s...</code>
				</div>
			</div>

			<div class="flex items-center justify-between text-xs text-slate-400 pt-1">
				<span class="flex items-center gap-1 text-slate-400">
					<svg class="w-3.5 h-3.5 text-amber-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
					Timeout Target: <strong class="text-slate-200 font-mono">Block #%d</strong>
				</span>
				<span class="font-mono text-slate-500">Created @ Height #%d</span>
			</div>
		</div>
		`, e.TxID, truncate(e.TxID, 12), statusBadge, e.Status, e.Amount, e.BuyerPubKeyHash, truncate(e.BuyerPubKeyHash, 10), e.SellerPubKeyHash, truncate(e.SellerPubKeyHash, 10), e.TimeoutBlock, e.CreatedAtHeight)
	}
	_, _ = fmt.Fprint(w, sb.String())
}

// RenderServicesPartial renders marketplace items
func RenderServicesPartial(w io.Writer, services []*indexer.AgentService) {
	if len(services) == 0 {
		_, _ = fmt.Fprint(w, `<div class="p-8 text-center text-slate-500 text-sm col-span-2 rounded-2xl bg-slate-900/40 border border-slate-800/80">No AI agent services registered yet. Register services via CLI or API.</div>`)
		return
	}

	var sb strings.Builder
	for _, s := range services {
		fmt.Fprintf(&sb, `
		<div class="p-5 rounded-2xl bg-slate-900/60 border border-slate-800/80 hover:border-emerald-500/40 transition-all flex flex-col justify-between space-y-4">
			<div class="space-y-2">
				<div class="flex items-start justify-between gap-3">
					<h3 class="font-outfit font-bold text-slate-100 text-base">%s</h3>
					<span class="px-2.5 py-1 bg-emerald-500/10 text-emerald-300 border border-emerald-500/30 text-xs font-extrabold font-outfit rounded-xl shadow-sm">
						%d μc / call
					</span>
				</div>
				<p class="text-xs text-slate-400 line-clamp-2 leading-relaxed">%s</p>
			</div>
			
			<div class="space-y-2.5 pt-3 border-t border-slate-800/60 text-xs">
				<div class="flex items-center justify-between text-slate-400">
					<span class="text-slate-500">Agent:</span>
					<code @click="copyToClipboard('%s')" class="font-mono text-indigo-300 bg-slate-950 px-2 py-0.5 rounded border border-slate-800 hover:border-indigo-500/50 cursor-pointer">%s</code>
				</div>
				<div class="flex items-center justify-between text-slate-400">
					<span class="text-slate-500">Endpoint:</span>
					<span class="font-mono text-cyan-400 hover:underline cursor-pointer truncate max-w-[180px]">%s</span>
				</div>
			</div>
		</div>
		`, s.Name, s.PricePerCall, s.Description, s.AgentAddress, truncate(s.AgentAddress, 14), s.EndpointURL)
	}
	_, _ = fmt.Fprint(w, sb.String())
}

// RenderFirewallPartial renders firewall budget usage
func RenderFirewallPartial(w io.Writer, sessionBudget, spent int64) {
	pct := 0
	if sessionBudget > 0 {
		pct = int((float64(spent) / float64(sessionBudget)) * 100)
		if pct > 100 {
			pct = 100
		}
	}
	remaining := sessionBudget - spent
	if remaining < 0 {
		remaining = 0
	}

	html := fmt.Sprintf(`
	<div class="glass-card rounded-3xl p-6 border border-slate-800/80 relative overflow-hidden group">
		<div class="absolute -right-10 -bottom-10 w-40 h-40 bg-indigo-600/10 rounded-full blur-3xl group-hover:bg-indigo-600/20 transition-all"></div>
		
		<div class="flex items-center justify-between mb-4">
			<div class="flex items-center gap-3">
				<div class="w-10 h-10 rounded-2xl bg-indigo-500/10 border border-indigo-500/20 flex items-center justify-center text-indigo-400">
					<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
				</div>
				<div>
					<h3 class="text-sm font-bold text-slate-200">Session Budget Limit</h3>
					<p class="text-xs text-slate-400">Policy Agent AI Micro-payments</p>
				</div>
			</div>
			<span class="px-3 py-1 rounded-full text-xs font-semibold bg-indigo-500/10 text-indigo-300 border border-indigo-500/20">
				%d%% Used
			</span>
		</div>

		<div class="space-y-2 mb-4">
			<div class="flex justify-between text-xs font-medium">
				<span class="text-slate-400">Spent: <strong class="text-slate-200">%d COIN</strong></span>
				<span class="text-slate-400">Budget: <strong class="text-slate-200">%d COIN</strong></span>
			</div>
			<div class="w-full bg-slate-900 rounded-full h-2.5 overflow-hidden p-0.5 border border-slate-800">
				<div class="bg-gradient-to-r from-indigo-500 via-purple-500 to-amber-500 h-1.5 rounded-full transition-all duration-500" style="width: %d%%"></div>
			</div>
		</div>

		<div class="grid grid-cols-2 gap-3 pt-3 border-t border-slate-800/60">
			<div class="p-3 rounded-xl bg-slate-950/60 border border-slate-800">
				<span class="text-slate-500 block text-[11px]">Available Budget</span>
				<strong class="text-emerald-400 font-outfit text-sm font-bold">%d COIN</strong>
			</div>
			<div class="p-3 rounded-xl bg-slate-950/60 border border-slate-800">
				<span class="text-slate-500 block text-[11px]">Passkey Firewall</span>
				<strong class="text-indigo-300 font-outfit text-sm font-bold flex items-center gap-1">
					<svg class="w-3.5 h-3.5 text-indigo-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"/></svg>
					ACTIVE
				</strong>
			</div>
		</div>
	</div>
	`, pct, spent, sessionBudget, pct, remaining)
	_, _ = fmt.Fprint(w, html)
}

// RenderBlocksPartial renders the recent block explorer stream
func RenderBlocksPartial(w io.Writer, blocks []*indexer.RecentBlock) {
	if len(blocks) == 0 {
		_, _ = fmt.Fprint(w, `<div class="p-6 text-center text-slate-500 text-sm rounded-2xl bg-slate-900/40 border border-slate-800/80">No recent blocks indexed yet.</div>`)
		return
	}

	var sb strings.Builder
	for _, b := range blocks {
		timeAgo := time.Since(b.Timestamp).Truncate(time.Second).String() + " ago"
		if time.Since(b.Timestamp) < 2*time.Second {
			timeAgo = "Just now"
		}

		fmt.Fprintf(&sb, `
		<div class="p-3.5 rounded-2xl bg-slate-900/60 border border-slate-800/80 hover:border-amber-500/40 transition-all flex items-center justify-between gap-3 text-xs">
			<div class="flex items-center gap-3">
				<span class="px-2.5 py-1 rounded-xl bg-amber-500/10 text-amber-300 border border-amber-500/20 font-extrabold font-outfit text-xs">
					#%d
				</span>
				<div>
					<div class="font-mono text-slate-200 font-semibold">%s...</div>
					<div class="text-[11px] text-slate-500 mt-0.5">%d transactions</div>
				</div>
			</div>
			<div class="text-right">
				<span class="text-slate-400 font-mono text-[11px] block">%s</span>
				<span class="text-[10px] text-emerald-400 font-medium bg-emerald-500/10 px-1.5 py-0.5 rounded border border-emerald-500/20 inline-block mt-0.5">CONFIRMED</span>
			</div>
		</div>
		`, b.Height, truncate(b.Hash, 14), b.TxCount, timeAgo)
	}
	_, _ = fmt.Fprint(w, sb.String())
}

func truncate(s string, l int) string {
	if len(s) <= l {
		return s
	}
	return s[:l]
}
