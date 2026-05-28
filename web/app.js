// WebSocket and API configuration
const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
const host = window.location.host;
const wsUrl = `${protocol}//${host}/ws/leaderboard/live`;
const apiUrl = `/api/v1/leaderboard`;

// Global State
let socket = null;
let leaderboardData = [];
let chart = null;

// DOM Elements
const connectionDot = document.getElementById('connection-dot');
const connectionText = document.getElementById('connection-text');
const leaderboardBody = document.getElementById('leaderboard-body');
const activityFeed = document.getElementById('activity-feed');

// Initialize Dashboard
document.addEventListener('DOMContentLoaded', () => {
    initChart();
    fetchLeaderboard();
    connectWebSocket();
});

// Fetch Initial Standings
async function fetchLeaderboard() {
    try {
        const response = await fetch(apiUrl);
        if (!response.ok) throw new Error('Network response was not ok');
        leaderboardData = await response.json();
        updateLeaderboardTable();
        updateChartData();
    } catch (err) {
        console.error('Error fetching leaderboard:', err);
        addActivityLog('System', 'Failed to load initial leaderboard from server.', 'system');
    }
}

// Connect to WebSocket with auto-reconnect
function connectWebSocket() {
    socket = new WebSocket(wsUrl);

    socket.onopen = () => {
        connectionDot.className = 'dot connected';
        connectionText.innerText = 'LIVE CONNECTED';
        addActivityLog('System', 'Live update connection established.', 'success');
    };

    socket.onmessage = (event) => {
        try {
            const update = JSON.parse(event.data);
            if (update.type === 'new_run') {
                const msg = `Team ${update.contestant_id} completed a new run! New Score: ${Math.round(update.score)}`;
                addActivityLog(update.contestant_id, msg, 'success');
                // Reload entire standings to ensure rankings are updated
                fetchLeaderboard();
            }
        } catch (err) {
            console.error('WS message parsing error:', err);
        }
    };

    socket.onclose = () => {
        connectionDot.className = 'dot disconnected';
        connectionText.innerText = 'RECONNECTING...';
        addActivityLog('System', 'Live update connection lost. Reconnecting...', 'system');
        setTimeout(connectWebSocket, 3000);
    };

    socket.onerror = (err) => {
        console.error('WS Error:', err);
    };
}

// Update Table DOM
function updateLeaderboardTable() {
    if (!leaderboardData || leaderboardData.length === 0) {
        leaderboardBody.innerHTML = `
            <tr class="placeholder-row">
                <td colspan="6">No submissions recorded yet. Be the first!</td>
            </tr>
        `;
        return;
    }

    leaderboardBody.innerHTML = '';
    leaderboardData.forEach((entry, index) => {
        const rank = index + 1;
        const tr = document.createElement('tr');
        tr.id = `sub-${entry.submission_id}`;

        let rankClass = `rank-val`;
        if (rank <= 3) {
            rankClass += ` rank-${rank}`;
        }

        tr.innerHTML = `
            <td class="${rankClass}">${rank}</td>
            <td class="team-name">${escapeHTML(entry.contestant_id)}</td>
            <td class="metric-val">${formatNumber(entry.tps)} orders/sec</td>
            <td class="latency-val">${formatNumber(entry.p99_latency_ms)} ms</td>
            <td class="success-val">${formatNumber(entry.success_rate)}%</td>
            <td class="score-val">${Math.round(entry.score)}</td>
        `;

        leaderboardBody.appendChild(tr);
    });
}

// Initialize Chart.js
function initChart() {
    const ctx = document.getElementById('performance-chart').getContext('2d');
    chart = new Chart(ctx, {
        type: 'bar',
        data: {
            labels: [],
            datasets: [
                {
                    label: 'TPS (Higher is Better)',
                    data: [],
                    backgroundColor: 'rgba(0, 242, 254, 0.4)',
                    borderColor: '#00f2fe',
                    borderWidth: 2,
                    yAxisID: 'y-tps',
                },
                {
                    label: 'p99 Latency (Lower is Better)',
                    data: [],
                    type: 'line',
                    borderColor: '#9b51e0',
                    backgroundColor: 'rgba(155, 81, 224, 0.1)',
                    borderWidth: 3,
                    pointBackgroundColor: '#9b51e0',
                    yAxisID: 'y-latency',
                }
            ]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                legend: {
                    labels: { color: '#9ca3af', font: { family: 'Inter' } }
                }
            },
            scales: {
                x: {
                    grid: { color: 'rgba(255,255,255,0.05)' },
                    ticks: { color: '#9ca3af', font: { family: 'Inter' } }
                },
                'y-tps': {
                    position: 'left',
                    grid: { color: 'rgba(255,255,255,0.05)' },
                    ticks: { color: '#00f2fe' },
                    title: { display: true, text: 'Throughput (TPS)', color: '#00f2fe' }
                },
                'y-latency': {
                    position: 'right',
                    grid: { drawOnChartArea: false },
                    ticks: { color: '#9b51e0' },
                    title: { display: true, text: 'p99 Latency (ms)', color: '#9b51e0' }
                }
            }
        }
    });
}

// Update Chart Data based on current leaderboard standings
function updateChartData() {
    if (!chart) return;

    // Compare the top 5 teams
    const topTeams = leaderboardData.slice(0, 5);
    
    chart.data.labels = topTeams.map(t => t.contestant_id);
    chart.data.datasets[0].data = topTeams.map(t => t.tps);
    chart.data.datasets[1].data = topTeams.map(t => t.p99_latency_ms);
    chart.update();
}

// Append Logs to Activity Feed
function addActivityLog(sender, msg, type = 'system') {
    const time = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
    const item = document.createElement('div');
    item.className = `feed-item ${type}`;

    item.innerHTML = `
        <span class="feed-time">${time}</span>
        <span class="feed-msg"><strong>${escapeHTML(sender)}:</strong> ${escapeHTML(msg)}</span>
    `;

    activityFeed.insertBefore(item, activityFeed.firstChild);

    // Limit logs length to 30
    if (activityFeed.children.length > 30) {
        activityFeed.removeChild(activityFeed.lastChild);
    }
}

// Helper Utilities
function formatNumber(num) {
    if (num === undefined || num === null) return '0';
    return Number(num).toLocaleString(undefined, { minimumFractionDigits: 0, maximumFractionDigits: 2 });
}

function escapeHTML(str) {
    return str.replace(/&/g, '&amp;')
              .replace(/</g, '&lt;')
              .replace(/>/g, '&gt;')
              .replace(/"/g, '&quot;')
              .replace(/'/g, '&#039;');
}
