/*
 * minimal
 *
 * Initializes Cocos Studio UI and scene readers. Sets up ccs.uiReader with 
 * widget parsers for 14 UI component types (ImageView, Button, CheckBox, etc.)
 * and ccs.sceneReader for scene graph management. Bootstraps the Cocos2d-x UI
 * framework by registering component readers with the loader.
 */
function minimal() {
    // Initialize UI reader with parser registry and utilities
    ccs.uiReader = {
        _fileDesignSizes: {},
        
        widgetFromJsonFile(filePath) {
            const res = cc.loader.getRes(cc.path.join(cc.loader.resPath, filePath));
            if (res) {
                this._fileDesignSizes[filePath] = cc.size(
                    res.designWidth || 0,
                    res.designHeight || 0
                );
            }
            
            const version = res.Version || res.version;
            const versionInt = ccs.uiReader.getVersionInteger(version);
            
            if (!version || versionInt >= 1700) {
                cc.warn("Not supported file types, Please try use the ccs.load");
                return null;
            }
            
            return ccs._load(filePath, "ccui");
        },
        
        registerTypeAndCallBack(typeName, widget, reader, callback) {
            const parser = ccs._load.getParser("ccui")["*"];
            const boundCallback = callback.bind(this);
            
            parser.registerParser(typeName, (data, path) => {
                const node = new widget();
                const options = data.options;
                
                if (reader.setPropsFromJsonDictionary) {
                    reader.setPropsFromJsonDictionary(node, options);
                }
                
                this.generalAttributes(node, options);
                
                let customProperty = options.customProperty;
                if (customProperty) {
                    customProperty = JSON.parse(customProperty);
                } else {
                    customProperty = {};
                }
                
                boundCallback(typeName, node, customProperty);
                this.colorAttributes(node, options);
                this.anchorPointAttributes(node, options);
                this.parseChild.call(this, node, data, path);
                
                return node;
            });
        },
        
        getVersionInteger(versionStr) {
            if (!versionStr || typeof versionStr !== "string") {
                return 0;
            }
            
            const parts = versionStr.split(".");
            if (parts.length !== 4) {
                return 0;
            }
            
            let versionInt = 0;
            parts.forEach((part, index) => {
                versionInt += part * Math.pow(10, 3 - index);
            });
            
            return versionInt;
        },
        
        storeFileDesignSize(filePath, size) {
            this._fileDesignSizes[filePath] = size;
        },
        
        getFileDesignSize(filePath) {
            return this._fileDesignSizes[filePath];
        },
        
        getFilePath() {
            return this._filePath;
        },
        
        setFilePath(filePath) {
            this._filePath = filePath;
        },
        
        getParseObjectMap() {
            return ccs._load.getParser("ccui")["*"].parsers;
        },
        
        getParseCallBackMap() {
            return ccs._load.getParser("ccui")["*"].parsers;
        },
        
        clear() {}
    };
    
    // Register UI component readers with parser
    const parser = ccs._load.getParser("ccui")["*"];
    
    ccs.imageViewReader = {
        setPropsFromJsonDictionary: parser.ImageViewAttributes
    };
    
    ccs.buttonReader = {
        setPropsFromJsonDictionary: parser.ButtonAttributes
    };
    
    ccs.checkBoxReader = {
        setPropsFromJsonDictionary: parser.CheckBoxAttributes
    };
    
    ccs.labelAtlasReader = {
        setPropsFromJsonDictionary: parser.TextAtlasAttributes
    };
    
    ccs.labelBMFontReader = {
        setPropsFromJsonDictionary: parser.TextBMFontAttributes
    };
    
    ccs.labelReader = {
        setPropsFromJsonDictionary: parser.TextAttributes
    };
    
    ccs.layoutReader = {
        setPropsFromJsonDictionary: parser.LayoutAttributes
    };
    
    ccs.listViewReader = {
        setPropsFromJsonDictionary: parser.ListViewAttributes
    };
    
    ccs.loadingBarReader = {
        setPropsFromJsonDictionary: parser.LoadingBarAttributes
    };
    
    ccs.pageViewReader = {
        setPropsFromJsonDictionary: parser.PageViewAttributes
    };
    
    ccs.scrollViewReader = {
        setPropsFromJsonDictionary: parser.ScrollViewAttributes
    };
    
    ccs.sliderReader = {
        setPropsFromJsonDictionary: parser.SliderAttributes
    };
    
    ccs.textFieldReader = {
        setPropsFromJsonDictionary: parser.TextFieldAttributes
    };
    
    // Initialize scene reader with node management
    ccs.sceneReader = {
        _node: null,
        
        createNodeWithSceneFile(filePath) {
            const node = ccs._load(filePath, "scene");
            this._node = node;
            return node;
        },
        
        getNodeByTag(tag) {
            if (this._node === null) {
                return null;
            }
            
            if (this._node.getTag() === tag) {
                return this._node;
            }
            
            return this._nodeByTag(this._node, tag);
        },
        
        _nodeByTag(parent, tag) {
            if (parent === null) {
                return null;
            }
            
            let result = null;
            const children = parent.getChildren();
            
            for (let i = 0; i < children.length; i++) {
                const child = children[i];
                
                if (child && child.getTag() === tag) {
                    result = child;
                    break;
                }
                
                result = this._nodeByTag(child, tag);
                if (result) {
                    break;
                }
            }
            
            return result;
        },
        
        version() {
            return "*";
        },
        
        setTarget() {},
        
        clear() {
            ccs.triggerManager.removeAll();
            cc.audioEngine.end();
        }
    };
}
