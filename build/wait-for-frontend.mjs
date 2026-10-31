import { setTimeout } from 'node:timers/promises';

const url = process.env.FRONTEND_DEVSERVER_URL;
if (!url) {
    throw new Error('FRONTEND_DEVSERVER_URL is missing. Start development with wails3 dev.');
}

const deadline = Date.now() + 120_000;
console.log(`Waiting for frontend at ${url}...`);

while (true) {
    try {
        const response = await fetch(url, { signal: AbortSignal.timeout(2_000) });
        await response.body?.cancel();
        if (response.ok) {
            console.log('Frontend is ready. Starting the app.');
            break;
        }
    } catch {
        // Vite may still be loading its configuration or preparing dependencies.
    }

    if (Date.now() >= deadline) {
        throw new Error(`Frontend did not become ready within 120 seconds: ${url}`);
    }
    await setTimeout(250);
}
