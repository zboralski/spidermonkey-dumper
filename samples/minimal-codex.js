/*
 * minimal
 *
 * Initializes the global `ccs.uiReader` and `ccs.sceneReader` registries.
 * `uiReader` loads CCUI JSON files, tracks per-file design sizes, parses version strings,
 * and registers per-widget constructors/readers with an optional callback hook.
 * `sceneReader` loads a scene file, stores the root node, and can search descendants by tag.
 */
function minimal() {
    const { ccs, cc } = globalThis;

    (function initUIReader() {
        const uiReader = {
            _fileDesignSizes: {},

            widgetFromJsonFile(file) {
                const json = cc.loader.getRes(cc.path.join(cc.loader.resPath, file));
                if (json) {
                    const designWidth = json.designWidth || 0;
                    const designHeight = json.designHeight || 0;
                    this._fileDesignSizes[file] = cc.size(designWidth, designHeight);
                }

                const version = (json && (json.Version || json.version)) || null;
                const versionInteger = this.getVersionInteger(version);

                // Matches bytecode behavior: missing version OR version >= 1700 is unsupported.
                if (!version || versionInteger >= 1700) {
                    cc.warn("Not supported file types, Please try use the ccs.load");
                    return null;
                }

                return ccs._load(file, "ccui");
            },

            registerTypeAndCallBack(type, WidgetCtor, reader, onParse) {
                const ccuiParser = ccs._load.getParser("ccui")["*"];
                const hook = typeof onParse === "function" ? onParse.bind(reader) : null;

                ccuiParser.registerParser(type, function parseWidget(data, extra) {
                    const widget = new WidgetCtor();
                    const options = data.options;

                    if (reader && reader.setPropsFromJsonDictionary) {
                        reader.setPropsFromJsonDictionary(widget, options);
                    }

                    this.generalAttributes(widget, options);

                    let customProps = options && options.customProperty;
                    customProps = customProps ? JSON.parse(customProps) : {};

                    if (hook) hook(type, widget, customProps);

                    this.colorAttributes(widget, options);
                    this.anchorPointAttributes(widget, options);

                    this.parseChild.call(this, widget, data, extra);
                    return widget;
                });
            },

            getVersionInteger(version) {
                if (!version || typeof version !== "string") return 0;

                const parts = version.split(".");
                if (parts.length !== 4) return 0;

                let acc = 0;
                parts.forEach((part, index) => {
                    acc += Number(part) * Math.pow(10, 3 - index);
                });
                return acc;
            },

            storeFileDesignSize(file, size) {
                this._fileDesignSizes[file] = size;
            },

            getFileDesignSize(file) {
                return this._fileDesignSizes[file];
            },

            getFilePath() {
                return this._filePath;
            },

            setFilePath(path) {
                this._filePath = path;
            },

            getParseObjectMap() {
                return ccs._load.getParser("ccui")["*"].parsers;
            },

            getParseCallBackMap() {
                return ccs._load.getParser("ccui")["*"].parsers;
            },

            clear() {},
        };

        ccs.uiReader = uiReader;

        const ccuiParser = ccs._load.getParser("ccui")["*"];

        ccs.imageViewReader = { setPropsFromJsonDictionary: ccuiParser.ImageViewAttributes };
        ccs.buttonReader = { setPropsFromJsonDictionary: ccuiParser.ButtonAttributes };
        ccs.checkBoxReader = { setPropsFromJsonDictionary: ccuiParser.CheckBoxAttributes };
        ccs.labelAtlasReader = { setPropsFromJsonDictionary: ccuiParser.TextAtlasAttributes };
        ccs.labelBMFontReader = { setPropsFromJsonDictionary: ccuiParser.TextBMFontAttributes };
        ccs.labelReader = { setPropsFromJsonDictionary: ccuiParser.TextAttributes };
        ccs.layoutReader = { setPropsFromJsonDictionary: ccuiParser.LayoutAttributes };
        ccs.listViewReader = { setPropsFromJsonDictionary: ccuiParser.ListViewAttributes };
        ccs.loadingBarReader = { setPropsFromJsonDictionary: ccuiParser.LoadingBarAttributes };
        ccs.pageViewReader = { setPropsFromJsonDictionary: ccuiParser.PageViewAttributes };
        ccs.scrollViewReader = { setPropsFromJsonDictionary: ccuiParser.ScrollViewAttributes };
        ccs.sliderReader = { setPropsFromJsonDictionary: ccuiParser.SliderAttributes };
        ccs.textFieldReader = { setPropsFromJsonDictionary: ccuiParser.TextFieldAttributes };
    })();

    (function initSceneReader() {
        ccs.sceneReader = {
            _node: null,

            createNodeWithSceneFile(file) {
                const node = ccs._load(file, "scene");
                this._node = node;
                return node;
            },

            getNodeByTag(tag) {
                if (this._node === null) return null;
                if (this._node.getTag() === tag) return this._node;
                return this._nodeByTag(this._node, tag);
            },

            _nodeByTag(node, tag) {
                if (node === null) return null;

                const children = node.getChildren();
                for (let i = 0; i < children.length; i++) {
                    const child = children[i];
                    if (child && child.getTag() === tag) return child;

                    const found = this._nodeByTag(child, tag);
                    if (found) return found;
                }
                return null;
            },

            version() {
                return "*";
            },

            setTarget() {},

            clear() {
                ccs.triggerManager.removeAll();
                cc.audioEngine.end();
            },
        };
    })();
}
