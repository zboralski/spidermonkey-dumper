/*
 * BaseScreen: Cocos2d-x UI base class with child synchronization, touch handling,
 * fog overlay management, and JSON-based screen loading. Extends cc.Layer to provide
 * automatic widget binding by name convention (btn_name → this.btnName), touch event
 * delegation, and dynamic layout adjustment for different screen sizes.
 */
const BaseScreen = cc.Layer.extend({
    screenConfig: null,
    fog: null,
    _clickEnable: true,
    _tintDark: null,
    _changeLocalize: true,
    _currId: "",
    _enableKeyboardListener: true,
    _isShowing: false,
    isLongTap: false,

    ctor() {
        this._super();
        this._tintDark = cc.tintTo(0.5, 140, 140, 140);
        this._tintDark.retain();
        return true;
    },

    syncAllChild(screenId) {
        this._currId = screenId;
        const resPath = "res/";
        this.screenConfig = ccs.load(resPath + screenId);
        this._rootNode = this.screenConfig.node;

        const nodeSize = this._rootNode.getContentSize();
        const designSize = cc.size(1280, 720);

        if (nodeSize.width >= designSize.width && nodeSize.height >= designSize.height) {
            const visibleSize = cc.director.getVisibleSize();
            this._rootNode.setContentSize(visibleSize);
            ccui.helper.doLayout(this._rootNode);
        }

        this._rootNode.setAnchorPoint(cc.p(0.5, 0.5));
        this._rootNode.setPosition(cc.p(
            this._rootNode.width / 2,
            this._rootNode.height / 2
        ));

        this.addChild(this._rootNode, 0);

        const children = this._rootNode.getChildren();
        this.syncAllChildHelper(children);
    },

    resyncAllChild(screenId) {
        if (this._rootNode == null) return;

        const children = this._rootNode.getChildren();
        for (let i = 0; i < children.length; i++) {
            if (children[i].parent != null) {
                children[i].parent.removeChild(children[i]);
            }
        }

        this.syncAllChild(screenId);
    },

    syncAllChildHelper(children) {
        if (children.length === 0) return;

        for (let i = 0; i < children.length; i++) {
            let name = children[i].getName();
            if (name === undefined) continue;

            const parts = name.split("_");
            if (parts.length > 2) {
                this[name] = children[i];
                continue;
            }

            name = parts[0] + parts[1];
            if (name in this) {
                this[name] = children[i];

                if (parts[0] === "btn") {
                    this[name].addTouchEventListener(this.onTouchEvent, this);
                }

                this.syncAllChildHelper(this[name].getChildren());
            }
        }
    },

    convertAlignCustomRichText(hAlign, vAlign) {
        switch (hAlign) {
            case cc.TEXT_ALIGNMENT_CENTER:
                hAlign = RichTextAlignment.CENTER;
                break;
            case cc.TEXT_ALIGNMENT_RIGHT:
                hAlign = RichTextAlignment.RIGHT;
                break;
            case cc.TEXT_ALIGNMENT_LEFT:
                hAlign = RichTextAlignment.LEFT;
                break;
        }

        switch (vAlign) {
            case cc.VERTICAL_TEXT_ALIGNMENT_TOP:
                vAlign = RichTextAlignment.TOP;
                break;
            case cc.VERTICAL_TEXT_ALIGNMENT_CENTER:
                vAlign = RichTextAlignment.MIDDLE;
                break;
            case cc.VERTICAL_TEXT_ALIGNMENT_BOTTOM:
                vAlign = RichTextAlignment.BOTTOM;
                break;
        }

        return cc.p(hAlign, vAlign);
    },

    createFog(opacity, touchEnabled) {
        this.fog = new ccui.Layout();
        this.fog.setBackGroundColorType(ccui.Layout.BG_COLOR_SOLID);
        this.fog.setBackGroundColor(cc.color.BLACK);
        this.fog.setContentSize(cc.size(cc.winSize.width, cc.winSize.height));
        this.fog.setPosition(cc.p(0, 0));

        if (opacity == null) {
            this.fog.setOpacity(180);
        } else {
            this.fog.setOpacity(255 * opacity);
        }

        if (touchEnabled == null) {
            this.fog.setTouchEnabled(true);
        } else {
            this.fog.setTouchEnabled(touchEnabled);
        }
    },

    showDisable(opacity, touchEnabled) {
        if (this.fog != null && this.fog.parent != null) {
            this.removeChild(this.fog);
            this.fog = null;
        }

        this.createFog(opacity, touchEnabled);
        this.addChild(this.fog, -1);
    },

    hideDisable() {
        if (this.fog == null) return;

        this.removeChild(this.fog);
        this.fog = null;
    },

    onTouchEvent(sender, eventType) {
        switch (eventType) {
            case ccui.Widget.TOUCH_BEGAN:
                this.onTouchBeganEvent(sender);
                break;
            case ccui.Widget.TOUCH_ENDED:
                this.onTouchEndEvent(sender);
                break;
            case ccui.Widget.TOUCH_CANCELED:
                this.onTouchCancelledEvent(sender);
                break;
            case ccui.Widget.TOUCH_MOVED:
                this.onTouchMovedEvent(sender);
                break;
        }
    },

    onTouchBeganEvent(sender) {
        sender.stopAllActions();
        sender.runAction(cc.sequence(this._tintDark));
    },

    onTouchEndEvent(sender) {
        sender.stopAllActions();
        sender.setColor(cc.color(255, 255, 255));
    },

    onTouchCancelledEvent(sender) {
        sender.playedSound = false;
        sender.stopAllActions();
        sender.setColor(cc.color(255, 255, 255));
    },

    onTouchMovedEvent(sender) {
    },

    showGui() {
        this._isShowing = true;
        if (this._changeLocalize) {
            this.localize();
        }
    },

    hideGui() {
        this._isShowing = false;
    }
});
