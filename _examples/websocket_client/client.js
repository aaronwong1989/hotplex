const WebSocket = require('ws');

// Correct route from main.go: /ws/v1/agent
const ws = new WebSocket('ws://localhost:8080/ws/v1/agent');

ws.on('open', function open() {
    console.log('[CLIENT] Connected to HotPlex (Port 8080)');

    // 1. Test Version Query
    console.log('[CLIENT] Sending version query...');
    ws.send(JSON.stringify({ type: 'version' }));

    // 2. Test Execution with System Prompt
    setTimeout(() => {
        const payload = {
            type: 'execute',
            session_id: 'v2-test-session',
            prompt: 'Print "System Prompt Test: OK".',
            system_prompt: 'You are a help bot. Answer briefly.',
            work_dir: '/tmp'
        };
        console.log('[CLIENT] Sending Execute:', payload);
        ws.send(JSON.stringify(payload));
    }, 1000);
});

ws.on('message', function incoming(data) {
    const message = JSON.parse(data);
    console.log('[SERVER EVENT]', message.event, JSON.stringify(message.data, null, 2));

    if (message.event === 'completed') {
        process.nextTick(() => {
            console.log('✅ Success! Requesting final stats...');
            ws.send(JSON.stringify({ type: 'stats', session_id: 'v2-test-session' }));
            setTimeout(() => ws.close(), 2000);
        });
    }
});

ws.on('error', (err) => {
    console.error('[CLIENT ERROR]', err.message);
});

ws.on('close', () => console.log('[CLIENT] Connection closed'));
