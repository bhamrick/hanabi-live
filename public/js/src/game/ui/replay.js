/*
    Functions for progressing forward and backward through time
*/

// Imports
const constants = require('../../constants');
const globals = require('./globals');
const stats = require('./stats');

/*
    Main replay functions
*/

const enter = () => {
    if (globals.inReplay) {
        return;
    }
    globals.inReplay = true;

    // Start by putting us at the end of the replay (the current game state)
    globals.replayPos = globals.replayLog.length;
    globals.replayTurn = globals.replayMax;

    // However, if the game just ended,
    // we want to go to the turn before the miscellaneous data sent at the end of the game
    if (globals.gameOver) {
        globals.replayPos = globals.finalReplayPos;
        globals.replayTurn = globals.finalReplayTurn;
    }

    // Hide the UI elements that overlap with the replay area
    globals.lobby.ui.stopAction();

    // Next, show the replay area and initialize some UI elements
    globals.elements.replayArea.show();
    adjustShuttles();
    setVisibleButtons();
    globals.layers.UI.batchDraw();
};
exports.enter = enter;

const exit = () => {
    if (!globals.inReplay) {
        return;
    }

    goto(globals.replayMax, true);
    globals.inReplay = false;
    globals.elements.replayArea.hide();

    if (globals.savedAction) {
        globals.lobby.ui.handleAction(globals.savedAction);
    }
    globals.elements.currentPlayerArea.setVisible(!globals.elements.clueArea.getVisible());
    if (globals.queuedAction !== null) {
        globals.elements.currentPlayerArea.hide();
        globals.elements.premoveCancelButton.show();
    }

    for (let i = 0; i < globals.deck.length; i++) {
        globals.deck[i].setBareImage();
    }

    globals.layers.UI.batchDraw();
    globals.layers.card.batchDraw();
};
exports.exit = exit;

const goto = (target, fast) => {
    // Validate function arguments
    if (target < 0) {
        target = 0;
    }
    if (target > globals.replayMax) {
        target = globals.replayMax;
    }
    if (target === globals.replayTurn) {
        return;
    }

    let rewind = false;
    if (target < globals.replayTurn) {
        rewind = true;
    }

    if (
        globals.sharedReplay
        && globals.amSharedReplayLeader
        && globals.useSharedTurns
    ) {
        shareCurrentTurn(target);
    }

    globals.replayTurn = target;

    setVisibleButtons();
    adjustShuttles();
    if (fast) {
        globals.animateFast = true;
    }

    if (rewind) {
        reset();
        globals.replayPos = 0;
    }

    // Iterate over the replay and stop at the current turn or at the end, whichever comes first
    while (true) {
        const msg = globals.replayLog[globals.replayPos];
        globals.replayPos += 1;

        // Stop at the end of the replay
        if (!msg) {
            break;
        }

        // Rebuild all notifies; this will correctly position cards and text
        globals.lobby.ui.handleNotify(msg);

        // Stop if you're at the current turn
        if (msg.type === 'turn' && msg.num === globals.replayTurn) {
            break;
        }
    }

    globals.animateFast = false;
    globals.elements.msgLogGroup.refreshText();
    globals.elements.messagePrompt.refreshText();
    globals.layers.card.batchDraw();
    globals.layers.UI.batchDraw();
};
exports.goto = goto;

const setVisibleButtons = () => {
    // If we are on the first turn, disable the rewind replay buttons
    globals.elements.replayBackFullButton.setEnabled(globals.replayTurn !== 0);
    globals.elements.replayBackButton.setEnabled(globals.replayTurn !== 0);

    // If we are on the last turn, disable the forward replay buttons
    globals.elements.replayForwardButton.setEnabled(globals.replayTurn !== globals.replayMax);
    globals.elements.replayForwardFullButton.setEnabled(globals.replayTurn !== globals.replayMax);
};

const reset = () => {
    // Reset some game state variables
    // "globals.turn" and "globals.currentPlayerIndex" is set in every "turn" command
    for (const card of globals.deck) {
        card.trueSuit = null;
        card.trueRank = null;
    }
    globals.deckSize = stats.getTotalCardsInTheDeck();
    // "globals.indexOfLastDrawnCard" is set in every "draw" command
    // "globals.score", "globals.maxScore", and "globals.clues"
    // are set in every "status" command
    globals.cardsGotten = 0;
    globals.cluesSpentPlusStrikes = 0;

    // Reset various UI elements
    globals.postAnimationLayout = null;
    globals.elements.messagePrompt.setMultiText('');
    globals.elements.msgLogGroup.reset();
    globals.elements.drawDeck.setCount(globals.deckSize);
    globals.elements.clueLog.clear();
    globals.elements.messagePrompt.reset();

    const { suits } = globals.variant;
    for (let i = 0; i < globals.elements.playerHands.length; i++) {
        globals.elements.playerHands[i].removeChildren();
    }
    for (const suit of suits) {
        globals.elements.playStacks.get(suit).removeChildren();
        globals.elements.discardStacks.get(suit).removeChildren();
    }
    for (const strike of globals.elements.strikes) {
        if (strike.tween) {
            strike.tween.destroy();
        }
        strike.setFaded();
    }

    globals.animateFast = true;
};

/*
    The 4 replay button functions
*/

exports.backFull = () => {
    checkDisableSharedTurns();
    goto(0, true);
};

exports.back = () => {
    checkDisableSharedTurns();
    goto(globals.replayTurn - 1, true);
};

exports.forward = () => {
    checkDisableSharedTurns();
    goto(globals.replayTurn + 1);
};

exports.forwardFull = () => {
    checkDisableSharedTurns();
    goto(globals.replayMax, true);
};

/*
    Extra replay functions
*/

exports.backRound = () => {
    checkDisableSharedTurns();
    goto(globals.replayTurn - globals.playerNames.length, true);
};

exports.forwardRound = () => {
    checkDisableSharedTurns();
    goto(globals.replayTurn + globals.playerNames.length);
};


/*
    The "Exit Replay" button
*/

exports.exitButton = () => {
    // Mark the time that the user clicked the "Exit Replay" button
    // (so that we can avoid an accidental "Give Clue" double-click)
    globals.accidentalClueTimer = Date.now();

    exit();
};

/*
    The replay shuttle
*/

exports.barClick = function barClick(event) {
    const rectX = event.evt.x - this.getAbsolutePosition().x;
    const w = this.getWidth();
    const step = w / globals.replayMax;
    const newTurn = Math.floor((rectX + step / 2) / step);
    if (newTurn !== globals.replayTurn) {
        checkDisableSharedTurns();
        goto(newTurn, true);
    }
};

exports.barDrag = function barDrag(pos) {
    const min = this.getParent().getAbsolutePosition().x;
    const w = this.getParent().getWidth() - this.getWidth();
    let shuttleX = pos.x - min;
    const shuttleY = this.getAbsolutePosition().y;
    if (shuttleX < 0) {
        shuttleX = 0;
    }
    if (shuttleX > w) {
        shuttleX = w;
    }
    const step = w / globals.replayMax;
    const newTurn = Math.floor((shuttleX + step / 2) / step);
    if (newTurn !== globals.replayTurn) {
        checkDisableSharedTurns();
        goto(newTurn, true);
    }
    shuttleX = newTurn * step;
    return {
        x: min + shuttleX,
        y: shuttleY,
    };
};

const positionReplayShuttle = (shuttle, turn) => {
    const w = shuttle.getParent().getWidth() - shuttle.getWidth();
    shuttle.setX(turn * w / globals.replayMax);
};

const adjustShuttles = () => {
    const shuttle = globals.elements.replayShuttle;
    const shuttleShared = globals.elements.replayShuttleShared;

    // If the shuttles are overlapping, then make the normal shuttle a little bit smaller
    let smaller = false;
    if (!globals.useSharedTurns && globals.replayTurn === globals.sharedReplayTurn) {
        smaller = true;
    }
    let size = 0.03;
    if (smaller) {
        size = 0.022;
    }
    const winW = globals.stage.getWidth();
    const winH = globals.stage.getHeight();
    shuttle.setWidth(size * winW);
    shuttle.setHeight(size * winH);

    // If it is smaller, we need to nudge it down a bit in order to center it
    let y = 0.0325 * winH;
    if (smaller) {
        const diffY = shuttleShared.getHeight() - shuttle.getHeight();
        y += diffY / 2;
    }
    shuttle.setY(y);

    // Adjust the shuttles along the X axis based on the current turn
    // If it is smaller, we need to nudge it to the right a bit in order to center it
    positionReplayShuttle(shuttleShared, globals.sharedReplayTurn);
    if (smaller) {
        const diffX = shuttleShared.getWidth() - shuttle.getWidth();
        const adjustment = diffX / 2;
        shuttle.setX(shuttleShared.getX() + adjustment);
    } else {
        positionReplayShuttle(shuttle, globals.replayTurn);
    }
};
exports.adjustShuttles = adjustShuttles;

/*
    Right-clicking the deck
*/

exports.promptTurn = (event) => {
    // Do nothing if this is not a right-click
    if (event.evt.which !== 3) {
        return;
    }

    let turn = window.prompt('Which turn do you want to go to?');
    if (Number.isNaN(turn)) {
        return;
    }
    turn -= 1;
    // We need to decrement the turn because
    // the turn shown to the user is always one greater than the real turn

    if (globals.replay) {
        checkDisableSharedTurns();
    } else {
        enter(true);
    }
    goto(turn, true);
};

/*
    The "Toggle Shared Turns" button
*/

exports.toggleSharedTurns = () => {
    globals.useSharedTurns = !globals.useSharedTurns;
    globals.elements.replayShuttleShared.setVisible(!globals.useSharedTurns);
    if (globals.useSharedTurns) {
        if (globals.amSharedReplayLeader) {
            shareCurrentTurn(globals.replayTurn);
        } else {
            goto(globals.sharedReplayTurn, true);
        }
    }

    // We need to adjust the shuttles in the case where
    // the normal shuttle is underneath the shared replay shuttle
    // and we need to make it bigger/smaller
    adjustShuttles();
};

// Navigating as a follower in a shared replay disables replay actions
const checkDisableSharedTurns = () => {
    if (
        globals.replay
        && globals.sharedReplay
        && !globals.amSharedReplayLeader
        && globals.useSharedTurns
    ) {
        // Replay actions are currently enabled, so disable them
        globals.elements.toggleSharedTurnButton.dispatchEvent(new MouseEvent('click'));
    }
};
exports.checkDisableSharedTurns = checkDisableSharedTurns;

const shareCurrentTurn = (target) => {
    if (globals.sharedReplayTurn === target) {
        return;
    }

    globals.lobby.conn.send('replayAction', {
        type: constants.REPLAY_ACTION_TYPE.TURN,
        turn: target,
    });
    globals.sharedReplayTurn = target;
    adjustShuttles();
};
