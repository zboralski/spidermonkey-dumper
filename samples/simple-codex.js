/*
 * simple
 *
 * Defines a Cocos2d-JS `SplashScene` that shows a splash UI and drives hot-update flow.
 * Fetches remote config (hotUpdate + api URLs), decides whether to update or launch game.
 * Uses `jsb.AssetsManager` events to check/update, persist search paths, and restart.
 * Updates a loading bar + label with either “Checking version” or “Updating” percent.
 */
function simple() {
    let stringHotUpdate;
    let stringAPI;
    let SplashScene;
    let Splash = null;

    SplashScene = cc.Scene.extend({
        _am: null,
        _storagePath: "",
        _updating: false,
        _updateListener: null,
        _progress: null,
        _isUpdateLobby: false,
        _loadingBar: null,

        count: 0,
        sprite: null,
        Splash: null,

        ctor() {
            this._super();

            Splash = this;
            this.Splash = this;

            if (cc.sys.isNative) {
                cc.game.onPassCheck = () => {
                    this.unscheduleAllCallbacks();
                    this.checkGame();
                };
            }

            this.count = 0;

            const winSize = cc.winSize;
            const bgPath = "res/res/GateImages/Loading/background.jpg";
            const hasLocalBg = !!(jsb?.fileUtils && jsb.fileUtils.isFileExist(bgPath));

            const backgroundSprite = new cc.Sprite(hasLocalBg ? bgPath : res.sprBg);
            backgroundSprite.setScale(winSize.width / backgroundSprite.getContentSize().width);
            backgroundSprite.x = winSize.width * 0.5;
            backgroundSprite.y = winSize.height * 0.5;

            const logoSprite = new cc.Sprite(res.sprLogo);
            logoSprite.setVisible(!hasLocalBg);
            logoSprite.x = winSize.width * 0.5;
            logoSprite.y = winSize.height * 0.5;

            this._loadingBar = new ccui.LoadingBar();
            this._loadingBar.loadTexture(res.loadBarX);

            const logoSize = logoSprite.getContentSize();
            this._loadingBar.setPosition(logoSize.width * 0.5, logoSize.height * 0.15);
            this._loadingBar.setPercent(0);
            logoSprite.addChild(this._loadingBar);

            this._progress = cc.LabelTTF.create("", res.FONTS_ARIALBD_TTF, 32);
            this._progress.enableStroke(cc.color(255, 255, 255), 2);
            this._progress.x = logoSize.width * 0.5;
            this._progress.y = -logoSize.height * 0.25;
            logoSprite.addChild(this._progress);

            this.addChild(backgroundSprite);
            this.addChild(logoSprite);

            const progressStep = 0.01;
            const tickSeconds = 0.02;
            this.schedule(() => {
                this.count += progressStep;
                this.updateProgress(this.count * 100);
                if (this.count >= 1) this.loadGame();
            }, tickSeconds);

            cc.game.onPassCheck?.();
        },

        checkCb(event) {
            switch (event.getEventCode()) {
                case jsb.EventAssetsManager.ERROR_NO_LOCAL_MANIFEST:
                case jsb.EventAssetsManager.ERROR_DOWNLOAD_MANIFEST:
                case jsb.EventAssetsManager.ERROR_PARSE_MANIFEST:
                    this.loadGame();
                    break;

                case jsb.EventAssetsManager.ALREADY_UP_TO_DATE:
                    this.inhangcathanhxuan();
                    break;

                case jsb.EventAssetsManager.NEW_VERSION_FOUND:
                    this._updating = false;
                    this.hotUpdate();
                    return;

                default:
                    return;
            }

            this._updating = false;
        },

        updateCb(event) {
            let shouldRestart = false;
            let shouldCleanup = false;

            switch (event.getEventCode()) {
                case jsb.EventAssetsManager.ERROR_NO_LOCAL_MANIFEST:
                    shouldCleanup = true;
                    break;

                case jsb.EventAssetsManager.UPDATE_PROGRESSION: {
                    const percent = event.getPercent();
                    this.updateProgress(percent);
                    break;
                }

                case jsb.EventAssetsManager.ERROR_DOWNLOAD_MANIFEST:
                case jsb.EventAssetsManager.ERROR_PARSE_MANIFEST:
                case jsb.EventAssetsManager.ALREADY_UP_TO_DATE:
                    shouldCleanup = true;
                    break;

                case jsb.EventAssetsManager.UPDATE_FINISHED:
                    shouldRestart = true;
                    break;

                case jsb.EventAssetsManager.UPDATE_FAILED:
                    this._updating = false;
                    this._canRetry = true;
                    this.retry();
                    break;

                case jsb.EventAssetsManager.ERROR_UPDATING:
                case jsb.EventAssetsManager.ERROR_DECOMPRESS:
                default:
                    break;
            }

            if (shouldCleanup) {
                cc.eventManager.removeListener(this._updateListener);
                this._updateListener = null;
                this._updating = false;
            }

            if (!shouldRestart) return;

            cc.eventManager.removeListener(this._updateListener);
            this._updateListener = null;

            const searchPaths = jsb.fileUtils.getSearchPaths();
            const newPaths = this._am.getLocalManifest().getSearchPaths();
            Array.prototype.unshift.apply(searchPaths, newPaths);

            cc.sys.localStorage.setItem("HotUpdateSearchPaths-JS", JSON.stringify(searchPaths));
            jsb.fileUtils.setSearchPaths(searchPaths);

            cc.game.restart();
        },

        hotUpdate() {
            if (!this._am || this._updating) return;

            this._updateListener = new jsb.EventListenerAssetsManager(
                this._am,
                this.updateCb.bind(this)
            );
            cc.eventManager.addListener(this._updateListener, 1);

            this._am.update();
            this._updating = true;
        },

        retry() {
            if (this._updating || !this._canRetry) return;
            this._canRetry = false;
            this._am.downloadFailedAssets();
        },

        inhangcathanhxuan() {
            cc.game.inhangcathanhxuan();
        },

        loadGame() {
            cc.game.loadGame();
        },

        checkGame() {
            GateRequestMoblie.get("https://ubiquitin.example.com/test-123/a.json", (state, body) => {
                if (state !== GateRequestMoblie.STATE.SUCCESS) {
                    this.loadGame();
                    return;
                }

                try {
                    const payload = JSON.parse(body);
                    stringHotUpdate = payload.hotUpdate;
                    stringAPI = payload.api;
                    mainGame.JSON_HOT_UPDATE = payload;
                } catch {
                    // Keep going with whatever values exist.
                }

                if (!stringHotUpdate) {
                    this.loadGame();
                    return;
                }

                if (GateRequestMoblie.checkDownloadLobby()) {
                    this.checkUpdate();
                    return;
                }

                const apiUrl = stringAPI;
                const alreadyChecked = fr.UserData.getBoolFromKey("hihihiczxczxc", false);

                if (!alreadyChecked) {
                    GateRequestMoblie.get(apiUrl, (state2, body2) => {
                        if (state2 !== GateRequestMoblie.STATE.SUCCESS) {
                            this.loadGame();
                            return;
                        }

                        const payload2 = JSON.parse(body2);
                        if (payload2.country !== "VN") {
                            this.loadGame();
                            return;
                        }

                        this.checkUpdate();
                    });
                } else {
                    this.checkUpdate();
                }
            });
        },

        checkUpdate() {
            fr.UserData.setBoolFromKey("hihihiczxczxc", true);

            if (!cc.sys.isNative) {
                this.loadGame();
                return;
            }

            this._storagePath = jsb?.fileUtils ? jsb.fileUtils.getWritablePath() : "./";

            const manifestStr = customManifestStrSrc(stringHotUpdate);
            this._am = new jsb.AssetsManager(manifestStr, this._storagePath, versionCompareHandle);
            this._am.retain();
            this._am.setVerifyCallback(() => true);

            if (cc.sys.os === cc.sys.OS_ANDROID) {
                this._am.setMaxConcurrentTask(2);
            }

            if (!this._am.getLocalManifest().isLoaded()) {
                this.loadGame();
                return;
            }

            const checkListener = new jsb.EventListenerAssetsManager(
                this._am,
                this.checkCb.bind(this)
            );
            cc.eventManager.addListener(checkListener, 1);

            this._am.checkUpdate();
        },

        updateProgress(percent) {
            cc.log("xxx vao day ", percent);

            const rounded = Math.round(percent);
            this._loadingBar.setPercent(rounded);

            if (!this._isUpdateLobby) {
                this._progress.string = `Checking version: ${rounded}%`;
            } else {
                this._progress.string = `Updating ${rounded}%`;
            }
        },

        onExit() {
            if (this._am) this._am.release();
            this._super();
        },
    });

    // Keep original side-effect semantics (no explicit return).
}
