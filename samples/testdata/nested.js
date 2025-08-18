/*
 * Function: BaseScreen
 * Behavior: [base screen function]
 */
function BaseScreen() {
    // call constructor on parent
    ccui.Widget.call(this);

    // create fog layer
    this.fog = new ccui.Layout();
    this.addChild(this.fog);
}

BaseScreen.prototype = Object.create(ccui.Widget.prototype);

// initialize member variables
BaseScreen.prototype._tintDark = false;
BaseScreen.prototype._super = null;
BaseScreen.prototype._currId = 0;
BaseScreen.prototype._isShowing = true;
BaseScreen.prototype._changeLocalize = true;

// constructor code from bytecode:
// this = BaseScreen();
// dup = new ccui.Widget()
// callprop _super on dup
// swap
// call 0
// pop

BaseScreen.prototype.showDisable = function() {
    if (this.fog != null && this.fog.parent != null) {
        // create fog layer with touch enabled and fade in
        var fog = new ccui.Layout();
        fog.setTouchEnabled(true);
        fog.setOpacity(127.49999999999999);

        // add to main screen
        this.addChild(fog, -1);

        // create fog content size to be winSize
        fog.setContentSize(cc.size(480, 320));

        // set position of fog in center of winSize
        fog.setPosition((winSize.width / 2), (winSize.height / 2));
    }

    this._isShowing = true;
};

BaseScreen.prototype.hideDisable = function() {
    if (this.fog != null) {
        this.removeChild(this.fog);
        this.fog = null;
    }
};

// more constructor code from bytecode:
// this = BaseScreen();
// getarg 1
// condswitch

BaseScreen.prototype.onTouchEvent = function(eventType) {
    switch (eventType) {
        case ccui.Widget.TOUCH_BEGAN:
            // on touch began event handler
            this.onTouchBeganEvent(eventType);
            break;
        case ccui.Widget.TOUCH_ENDED:
            // on touch end event handler
            this.onTouchEndEvent(eventType);
            break;
        case ccui.Widget.TOUCH_CANCELED:
            // on touch canceled event handler
            this.onTouchCancelledEvent(eventType);
            break;
        default:
            // on touch moved event handler
            this.onTouchMovedEvent();
    }
};

BaseScreen.prototype.onTouchBeganEvent = function(eventType) {
    // stop all actions
    this.stopAllActions();

    // run action sequence with tint effect
    var seq = new cc.Sequence();
    this.runAction(seq);
};

// more bytecode code:
// getarg 0
// dup
// callprop _stopAllActions
// swap
// call 0

BaseScreen.prototype.onTouchEndEvent = function(eventType) {
    // stop all actions
    this.stopAllActions();

    // set color to white
    var seq = new cc.Sequence();
    this.setColor(cc.color(255, 255, 255));
};

// more bytecode code:
// getarg 0
// dup
// callprop _stopAllActions
// swap
// call 3

BaseScreen.prototype.onTouchCancelledEvent = function(eventType) {
    // stop all actions
    this.stopAllActions();

    // set color to white
    var seq = new cc.Sequence();
    this.setColor(cc.color(255, 255, 255));
};

// more bytecode code:
// retrval