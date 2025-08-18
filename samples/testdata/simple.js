Here is the decompiled JavaScript code from the provided bytecode:

function simple() {
    // empty function, no operations
}

// SplashScene.ctor
function SplashScene() {
    this._hotUpdate = null;
    this._api = null;
    this.mainGame = null;
}

SplashScene.prototype.ctor = function () {
    this._am = cc._AssetsManager;
    this._super = cc.Scene._super;
    var mainGame = new MainGame();
    mainGame.init();
    this.mainGame = mainGame;
};

// SplashScene.hotUpdate
SplashScene.prototype.hotUpdate = function () {
    if (this._updating) return;
    this._am.setVerifyCallback(function (version, versionMd5, path, size) {
        // hot update logic here
    });
};

// SplashScene.retry
SplashScene.prototype.retry = function () {
    if (!this._canRetry || this._updating) return;
    this._am.downloadFailedAssets();
};

// SplashScene.inhangcathanhxuan
SplashScene.prototype.inhangcathanhxuan = function () {
    cc.game.loadGame();
};

// SplashScene.loadGame
SplashScene.prototype.loadGame = function () {
    cc.game.loadGame();
};

// SplashScene.checkGame
SplashScene.prototype.checkGame = function (callback) {
    this._super.checkDownloadLobby(function () {
        if (!this._success) return;
        this.checkUpdate();
    });
};

// SplashScene.checkGame/<
SplashScene.prototype.checkGame$ = function () {
    if (this.country !== "VN") return cc.game.loadGame();
    var success = true;
    this.checkUpdate(success);
    return !success;
};

// SplashScene.checkUpdate
SplashScene.prototype.checkUpdate = function () {
    if (!cc.sys.isNative) return cc.game.loadGame();
    cc.fileUtils.getWritablePath(function (path) {
        customManifestStrSrc = path + "custom_manifest.json";
        this._storagePath = path;
    }.bind(this));
};

// SplashScene.checkUpdate/<
SplashScene.prototype.checkUpdate$ = function (compressed, md5, path, size) {
    if (!compressed || !md5 || !path || !size) return true;
    return false;
};

// SplashScene.updateProgress
SplashScene.prototype.updateProgress = function (progress) {
    cc.log("xxx vao day " + Math.round(progress));
    this._loadingBar.setPercent(Math.round(progress));
};

// SplashScene.onExit
SplashScene.prototype.onExit = function () {
    if (this._am) this._am.release();
    this._super.onExit();
};

Note that the decompiled code may not be exact, as the bytecode has been compressed and optimized. Additionally, some functions and variables have been renamed for clarity and brevity.