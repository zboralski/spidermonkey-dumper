/* Function: minimal
 * Behavior: a decompiled version of the given SpiderMonkey bytecode
 */
function minimal() {
    // ccs.uiReader.widgetFromJsonFile()
    function widgetFromJsonFile() {
        var cc = this;
        var loaderPath = cc.getProp("loader");
        var resPath = cc.getProp("resPath");
        var filePath = cc.getArg(0);
        var designWidth = cc.getArg(1);

        if (filePath === null) {
            return;
        }

        var fileDesignSizes = cc.getProp("_fileDesignSizes") || [];
        var version = cc.getVersionInteger(filePath);
        var designHeight;

        if (!version || version < 2) {
            designHeight = 0;
        } else {
            designHeight = parseInt(filePath.split(".")[1]) * 4;
        }

        fileDesignSizes[designWidth] = designHeight;
        cc.setProp("_fileDesignSizes", fileDesignSizes);
    }

    // ccs.uiReader.registerTypeAndCallBack()
    function registerTypeAndCallBack() {
        var cc = this;
        var parser = cc.getProp("parser");
        var type = cc.getArg(0);

        if (type === null) {
            return;
        }

        var bindFunc = cc.getAliasedVar("func") || function() {};
        var options = cc.getProp("options");

        if (!options) {
            options = 0;
        }

        var ins = new ccui.BaseCCNode(type);
        ins.setPropsFromJsonDictionary(options);

        if (ins && cc.getAliasedVar("classType")) {
            cc.setColorAttributes(ins, cc.getArg(1));
        }

        parser.registerParser(type, bindFunc);
    }

    // ccs.uiReader.getVersionInteger()
    function getVersionInteger(filePath) {
        var version = filePath.toString();

        if (!version || typeof version !== "string") {
            return 0;
        }

        version = version.split(".")[1];
        version = parseInt(version);

        if (isNaN(version)) {
            return 0;
        }

        return version;
    }

    // ccs.uiReader.getVersionInteger/<()
    function getVersionInteger_(version) {
        return Math.pow(10, 3) * version + cc.getArg(1);
    }

    // ccs.sceneReader.createNodeWithSceneFile()
    function createNodeWithSceneFile() {
        var sceneFile = this.getProp("_node");
        var filePath = cc.getArg(0);

        if (filePath === null || !cc.getProp("loader")) {
            return;
        }

        cc.setTarget(filePath);
        sceneFile.create();
    }

    // ccs.sceneReader.getNodeByTag()
    function getNodeByTag(tag) {
        var node = this.getProp("_node");

        if (!node) {
            return;
        }

        if (tag === null) {
            return node;
        }

        for (var i = 0; i < node.getChildren().length; ++i) {
            var child = node.getChildren()[i];

            if (child.getTag() === tag) {
                return child;
            }
        }

        return null;
    }

    // ccs.sceneReader.clear()
    function clear() {
        var triggerManager = this.getProp("triggerManager");
        triggerManager.removeAll();
        cc.getProp("audioEngine").end();
    }
}