/*
 * Initializes audio engine constants on the jsb.AudioEngine object.
 * Defines AudioState enum (ERROR=-1, INITIALIZING=0, PLAYING=1, PAUSED=2)
 * and sentinel constants INVALID_AUDIO_ID=-1, TIME_UNKNOWN=-1.
 * Early returns if jsb or jsb.AudioEngine is missing.
 */
function constants(jsb) {
    if (!jsb || !jsb.AudioEngine) {
        return undefined;
    }

    jsb.AudioEngine.AudioState = {
        ERROR: -1,
        INITIALIZING: 0,
        PLAYING: 1,
        PAUSED: 2
    };

    jsb.AudioEngine.INVALID_AUDIO_ID = -1;
    jsb.AudioEngine.TIME_UNKNOWN = -1;
}
