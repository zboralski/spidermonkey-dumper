/*
 * nested
 *
 * Defines a Cocos2d-JS `BaseScreen` layer class with helpers to load a UI layout,
 * bind child widgets onto `this` by name, and wire button touch events.
 * Provides a semi-transparent “fog” overlay to disable interaction, plus show/hide
 * state and optional localization hook. (A few numeric constants are inferred.)
 */
function nested() {
    const DEFAULT_ANCHOR = 0.5;

    // Inferred: duration for tint action, and fog opacity constants.
    const TINT_DURATION_SECONDS = 0.1;
    const DEFAULT_FOG_OPACITY = 150;
    const MAX_OPACITY = 255;

    var BaseScreen = cc.Layer.extend({
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

            this._tintDark = cc.tintTo(TINT_DURATION_SECONDS, 140, 140, 140);
            this._tintDark.retain();

            return true;
        },

        syncAllChild(layoutId) {
            this._currId = layoutId;

            const basePath = "res/";
            this.screenConfig = ccs.load(basePath + layoutId);
            this._rootNode = this.screenConfig.node;

            const contentSize = this._rootNode.getContentSize();
            const designSize = cc.size(1280, 720);

            if (contentSize.width >= designSize.width && contentSize.height >= designSize.height) {
                const visibleSize = cc.director.getVisibleSize();
                this._rootNode.setContentSize(visibleSize);
                ccui.helper.doLayout(this._rootNode);
            }

            this._rootNode.setAnchorPoint(cc.p(DEFAULT_ANCHOR, DEFAULT_ANCHOR));
            this._rootNode.setPosition(
                cc.p(this._rootNode.width / 2, this._rootNode.height / 2),
            );

            this.addChild(this._rootNode, 0);

            const children = this._rootNode.getChildren();
            this.syncAllChildHelper(children);
        },

        resyncAllChild(layoutId) {
            if (this._rootNode == null) return;

            const children = this._rootNode.getChildren();
            for (let i = 0; i < children.length; i++) {
                const child = children[i];
                if (child.parent != null) {
                    child.parent.removeChild(child);
                }
            }

            this.syncAllChild(layoutId);
        },

        syncAllChildHelper(children) {
            if (children.length === 0) return;

            for (let i = 0; i < children.length; i++) {
                const child = children[i];
                const rawName = child.getName();
                if (rawName === undefined) continue;

                const parts = rawName.split("_");

                // If the name has 3+ segments, store under the full name directly.
                if (parts.length > 2) {
                    this[rawName] = child;
                    continue;
                }

                // Otherwise build a key from the first two segments.
                const key = `${parts[0]}${parts[1]}`;

                // Only bind if the key already exists somewhere on `this`.
                if (!(key in this)) continue;

                this[key] = child;

                if (parts[0] === "btn") {
                    this[key].addTouchEventListener(this.onTouchEvent, this);
                }

                this.syncAllChildHelper(this[key].getChildren());
            }
        },

        convertAlignCustomRichText(horizontalAlign, verticalAlign) {
            switch (horizontalAlign) {
                case cc.TEXT_ALIGNMENT_CENTER:
                    horizontalAlign = RichTextAlignment.CENTER;
                    break;
                case cc.TEXT_ALIGNMENT_RIGHT:
                    horizontalAlign = RichTextAlignment.RIGHT;
                    break;
                case cc.TEXT_ALIGNMENT_LEFT:
                    horizontalAlign = RichTextAlignment.LEFT;
                    break;
                default:
                    break;
            }

            switch (verticalAlign) {
                case cc.VERTICAL_TEXT_ALIGNMENT_TOP:
                    verticalAlign = RichTextAlignment.TOP;
                    break;
                case cc.VERTICAL_TEXT_ALIGNMENT_CENTER:
                    verticalAlign = RichTextAlignment.MIDDLE;
                    break;
                case cc.VERTICAL_TEXT_ALIGNMENT_BOTTOM:
                    verticalAlign = RichTextAlignment.BOTTOM;
                    break;
                default:
                    break;
            }

            return cc.p(horizontalAlign, verticalAlign);
        },

        createFog(opacityScale, touchEnabled) {
            this.fog = new ccui.Layout();

            this.fog.setBackGroundColorType(ccui.Layout.BG_COLOR_SOLID);
            this.fog.setBackGroundColor(cc.color.BLACK);
            this.fog.setContentSize(cc.size(cc.winSize.width, cc.winSize.height));
            this.fog.setPosition(cc.p(0, 0));

            if (opacityScale == null) {
                this.fog.setOpacity(DEFAULT_FOG_OPACITY);
            } else {
                this.fog.setOpacity(MAX_OPACITY * opacityScale);
            }

            if (touchEnabled == null) {
                this.fog.setTouchEnabled(true);
            } else {
                this.fog.setTouchEnabled(touchEnabled);
            }
        },

        showDisable(opacityScale, touchEnabled) {
            if (this.fog != null && this.fog.parent != null) {
                this.removeChild(this.fog);
                this.fog = null;
            }

            this.createFog(opacityScale, touchEnabled);
            this.addChild(this.fog, -1);
        },

        hideDisable() {
            if (this.fog == null) return;

            this.removeChild(this.fog);
            this.fog = null;
        },

        onTouchEvent(sender, touchType) {
            switch (touchType) {
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
                default:
                    break;
            }
        },

        onTouchBeganEvent(widget) {
            widget.stopAllActions();
            widget.runAction(cc.sequence(this._tintDark));
        },

        onTouchEndEvent(widget) {
            widget.stopAllActions();
            widget.setColor(cc.color(255, 255, 255));
        },

        onTouchCancelledEvent(widget) {
            widget.playedSound = false;

            widget.stopAllActions();
            widget.setColor(cc.color(255, 255, 255));
        },

        onTouchMovedEvent() {
            // no-op
        },

        showGui() {
            this._isShowing = true;

            if (this._changeLocalize) {
                this.localize();
            }
        },

        hideGui() {
            this._isShowing = false;
        },
    });
}
