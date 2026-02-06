/*
 * constants
 *
 * Initializes audio-related constants on `jsb.AudioEngine` if present.
 * Creates an `AudioState` enum-like object and sets sentinel constants.
 * Inputs: global `jsb` object; Output: none (returns `undefined`).
 */
function constants() {
    (function initAudioConstants(jsb) {
        if (!jsb || !jsb.AudioEngine) return;

        const { AudioEngine } = jsb;

        AudioEngine.AudioState = {
            ERROR: -1,
            INITIALIZING: 0,
            PLAYING: 1,
            PAUSED: 2,
        };

        AudioEngine.INVALID_AUDIO_ID = -1;
        AudioEngine.TIME_UNKNOWN = -1;
    })(jsb);
}
