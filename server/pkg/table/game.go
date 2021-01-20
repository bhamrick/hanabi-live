package table

import (
	"time"

	"github.com/Zamiell/hanabi-live/server/pkg/constants"
	"github.com/Zamiell/hanabi-live/server/pkg/options"
	"github.com/Zamiell/hanabi-live/server/pkg/variants"
)

// game is a sub-object of a table.
// It represents all of the particular state associated with a game.
// We need to export most fields so that the JSON encoder can serialize them during a graceful
// server restart.
// (Several fields must be skipped in order to prevent circular references.)
type game struct {
	// This is a reference to the parent object; every game must have a parent Table object
	table *table
	// These are references to the respective fields of the Table object (for convenience purposes)
	options      *options.Options
	extraOptions *options.ExtraOptions
	variant      *variants.Variant

	// This corresponds to the database field of "datetime_started"
	// It will be equal to "Table.DatetimeStarted" in an ongoing game that has not been written to
	// the database yet
	DatetimeStarted time.Time

	// This corresponds to the database field of "datetime_finished"
	// It will be blank in an ongoing game that has not been written to the database yet
	DatetimeFinished time.Time

	// Game state related fields
	Players []*gamePlayer
	// The seed specifies how the deck is dealt
	// It is either entered manually by players before the game starts or randomly selected by the
	// server upon starting a game
	Seed                string
	Deck                []*card
	CardIdentities      []*options.CardIdentity // A bare-bones version of the deck
	DeckIndex           int
	Stacks              []int
	PlayStackDirections []int // The values for this are listed in "constants.go"
	Turn                int   // Starts at 0; the client will represent turn 0 as turn 1 to the user
	DatetimeTurnBegin   time.Time
	TurnsInverted       bool
	ActivePlayerIndex   int // Every game always starts with the 0th player going first
	ClueTokens          int
	Score               int
	MaxScore            int
	Strikes             int
	LastClueTypeGiven   int // Used in "Alternating Clues" variants
	// Actions is a list of all of the in-game moves that players have taken thus far
	// Different actions will have different fields, so we need this to be an generic interface
	// Furthermore, we do not want this to be a pointer of interfaces because
	// this simplifies action scrubbing
	Actions []interface{}
	// DBActions is a database-compatible representation of in-game moves
	// (it is much less verbose when compared with Actions)
	DBActions             []*options.GameAction
	InvalidActionOccurred bool // Used when emulating game actions in replays
	EndCondition          int  // The values for this are listed in "constants.go"
	// The index of the player who ended the game, if any
	// (needed for writing a "game over" terminate action to the database)
	EndPlayer int
	// Initialized to -1 and set when the final card is drawn
	// (to determine when the game should end)
	EndTurn int

	// Time & Pause related fields
	StartedTimer     bool // The timer is only started when the initial player has finished loading
	Paused           bool
	PausePlayerIndex int
	PauseCount       int

	// Shared replay fields
	EfficiencyMod int

	// Hypothetical-related fields
	Hypothetical       bool // Whether or not we are in a post-game hypothetical
	HypoActions        []string
	HypoShowDrawnCards bool // Whether or not drawn cards should be revealed (false by default)

	// Keep track of user-defined tags; they will be written to the database upon game completion
	Tags map[string]int // Keys are the tags, values are the user ID that created it
}

func (m *Manager) newGame(t *table) *game {
	g := &game{
		table:        t,
		options:      t.Options,
		extraOptions: t.ExtraOptions,
		variant:      t.Variant,

		DatetimeStarted:  time.Time{},
		DatetimeFinished: time.Time{},

		Players:               make([]*gamePlayer, 0),
		Seed:                  "",
		Deck:                  make([]*card, 0),
		CardIdentities:        make([]*options.CardIdentity, 0),
		DeckIndex:             0,
		Stacks:                make([]int, len(t.Variant.Suits)),
		PlayStackDirections:   make([]int, len(t.Variant.Suits)),
		Turn:                  0,
		DatetimeTurnBegin:     time.Now(),
		TurnsInverted:         false,
		ActivePlayerIndex:     0,
		ClueTokens:            t.Variant.GetAdjustedClueTokens(constants.MaxClueNum),
		Score:                 0,
		MaxScore:              len(t.Variant.Suits) * constants.PointsPerSuit,
		Strikes:               0,
		LastClueTypeGiven:     -1,
		Actions:               make([]interface{}, 0),
		DBActions:             make([]*options.GameAction, 0),
		InvalidActionOccurred: false,
		EndCondition:          0,
		EndPlayer:             -1,
		EndTurn:               -1,

		StartedTimer:     false,
		Paused:           false,
		PausePlayerIndex: -1,
		PauseCount:       0,

		EfficiencyMod: 0,

		Hypothetical:       false,
		HypoActions:        make([]string, 0),
		HypoShowDrawnCards: false,

		Tags: make(map[string]int),
	}

	// Reverse the stack direction of reversed suits,
	// except on the "Up or Down" variant that uses the "Undecided" direction
	if t.Variant.HasReversedSuits() && !t.Variant.IsUpOrDown() {
		for i, s := range t.Variant.Suits {
			if s.Reversed {
				g.PlayStackDirections[i] = variants.StackDirectionDown
			} else {
				g.PlayStackDirections[i] = variants.StackDirectionUp
			}
		}
	}

	return g
}

// ---------------
// Major functions
// ---------------

/*

// CheckTimer is meant to be called in a new goroutine
func (g *Game) CheckTimer(
	ctx context.Context,
	timeToSleep time.Duration,
	turn int,
	pauseCount int,
	gp *GamePlayer,
) {
	// Sleep until the active player runs out of time
	time.Sleep(timeToSleep)

	// Local variables
	t := g.Table

	// Check to see if the table still exists
	t2, exists := getTableAndLock(ctx, nil, t.ID, false, true)
	if !exists || t != t2 {
		return
	}
	t.Lock(ctx)
	defer t.Unlock(ctx)

	// Check to see if we have made a move in the meanwhile
	if turn != g.Turn {
		return
	}

	// Check to see if the game is currently paused
	if g.Paused {
		return
	}

	// Check to see if the game was paused while we were sleeping
	if pauseCount != g.PauseCount {
		return
	}

	// Check to see if the game ended already
	if g.EndCondition > constants.EndConditionInProgress {
		return
	}

	g.EndTimer(ctx, gp)
}

// EndTimer is called when a player has run out of time in a timed game, which will automatically
// end the game with a score of 0
// The table lock is assumed to be acquired in this function
func (g *Game) EndTimer(ctx context.Context, gp *GamePlayer) {
	// Local variables
	t := g.Table

	hLog.Infof("%v Time ran out for: %v", t.GetName(), gp.Name)

	// Adjust the final player's time (for the purposes of displaying the correct ending times)
	gp.Time = 0

	// Get the session of this player
	p := t.Players[gp.Index]
	s := p.Session
	if s == nil {
		// A player's session should never be nil
		// They might be in the process of reconnecting,
		// so make a fake session that will represent them
		s = NewFakeSession(p.UserID, p.Name)
		hLog.Info("Created a new fake session.")
	}

	// End the game
	commandAction(ctx, s, &CommandData{ // nolint: exhaustivestruct
		TableID:     t.ID,
		Type:        ActionTypeEndGame,
		Target:      gp.Index,
		Value:       EndConditionTimeout,
		NoTableLock: true,
	})
}

// CheckEnd examines the game state and sets "EndCondition" to the appropriate value, if any
func (g *Game) CheckEnd() bool {
	// Local variables
	t := g.Table
	variant := variants[g.Options.VariantName]

	// Some ending conditions will already be set by the time we get here
	if g.EndCondition == EndConditionTimeout ||
		g.EndCondition == EndConditionTerminated ||
		g.EndCondition == EndConditionIdleTimeout ||
		g.EndCondition == EndConditionCharacterSoftlock {

		return true
	}

	// Check for 3 strikes
	if g.Strikes == MaxStrikeNum {
		hLog.Infof("%v 3 strike maximum reached; ending the game.", t.GetName())
		g.EndCondition = EndConditionStrikeout
		return true
	}

	// In a speedrun, check to see if a perfect score can still be achieved
	if g.Options.Speedrun && g.MaxScore < variant.MaxScore {
		hLog.Infof("%v A perfect score is impossible in a speedrun; ending the game.", t.GetName())
		g.EndCondition = EndConditionSpeedrunFail
		return true
	}

	// In an "All or Nothing" game, check to see if a maximum score can still be reached
	if g.Options.AllOrNothing && g.MaxScore < variant.MaxScore {
		hLog.Infof(
			"%v A perfect score is impossible in an \"All or Nothing\" game; ending the game.",
			t.GetName(),
		)
		g.EndCondition = EndConditionAllOrNothingFail
		return true
	}

	// In an "All or Nothing game",
	// handle the case where a player would have to discard without any cards in their hand
	if g.Options.AllOrNothing &&
		len(g.Players[g.ActivePlayerIndex].Hand) == 0 &&
		g.ClueTokens < variant.GetAdjustedClueTokens(1) {

		hLog.Infof(
			"%v The current player has no cards and no clue tokens in an \"All or Nothing\" game; ending the game.",
			t.GetName(),
		)
		g.EndCondition = EndConditionAllOrNothingSoftlock
		g.EndPlayer = g.Players[g.ActivePlayerIndex].Index
		return true
	}

	// Check to see if the final go-around has completed
	// (which is initiated after the last card is played from the deck)
	if g.Turn == g.EndTurn {
		hLog.Infof("%v Final turn reached; ending the game.", t.GetName())
		g.EndCondition = EndConditionNormal
		return true
	}

	// Check to see if the maximum score has been reached
	if g.Score == g.MaxScore {
		hLog.Infof("%v Maximum score reached; ending the game.", t.GetName())
		g.EndCondition = EndConditionNormal
		return true
	}

	// Check to see if there are any cards remaining that can be played on the stacks
	if variant.HasReversedSuits() {
		// Searching for the next card is much more complicated if we are playing an "Up or Down"
		// or "Reversed" variant, so the logic for this is stored in a separate file
		if !variantReversibleCheckAllDead(g) {
			return false
		}
	} else {
		for i, stackLen := range g.Stacks {
			// Search through the deck
			if stackLen == 5 {
				continue
			}
			neededSuit := i
			neededRank := stackLen + 1
			for _, c := range g.Deck {
				if c.SuitIndex == neededSuit &&
					c.Rank == neededRank &&
					!c.Discarded &&
					!c.CannotBePlayed {

					return false
				}
			}
		}
	}

	// If we got this far, nothing can be played
	hLog.Infof("%v No remaining cards can be played; ending the game.", t.GetName())
	g.EndCondition = EndConditionNormal
	return true
}

*/

// -----------------------
// Miscellaneous functions
// -----------------------

/*

func (g *Game) GetHandSize() int {
	handSize := g.GetHandSizeForNormalGame()
	if g.Options.OneExtraCard {
		handSize++
	}
	if g.Options.OneLessCard {
		handSize--
	}
	return handSize
}

func (g *Game) GetHandSizeForNormalGame() int {
	// Local variables
	t := g.Table
	numPlayers := len(g.Players)

	if numPlayers == 2 || numPlayers == 3 {
		return 5
	} else if numPlayers == 4 || numPlayers == 5 {
		return 4
	} else if numPlayers == 6 {
		return 3
	}

	hLog.Errorf("Failed to get the hand size for %v players for table: %v", numPlayers, t.Name)
	return 4
}

// GetMaxScore calculates what the maximum score is,
// accounting for stacks that cannot be completed due to discarded cards
func (g *Game) GetMaxScore() int {
	// Local variables
	variant := variants[g.Options.VariantName]

	// Getting the maximum score is much more complicated if we are playing a
	// "Reversed" or "Up or Down" variant
	if variant.HasReversedSuits() {
		return variantReversibleGetMaxScore(g)
	}

	maxScore := 0
	for suit := range g.Stacks {
		for rank := 1; rank <= 5; rank++ {
			// Search through the deck to see if all the copies of this card are discarded already
			total, discarded := g.GetSpecificCardNum(suit, rank)
			if total > discarded {
				maxScore++
			} else {
				break
			}
		}
	}

	return maxScore
}

// GetSpecificCardNum returns the total cards in the deck of the specified suit and rank
// as well as how many of those that have been already discarded
func (g *Game) GetSpecificCardNum(suitIndex int, rank int) (int, int) {
	total := 0
	discarded := 0
	for _, c := range g.Deck {
		if c.SuitIndex == suitIndex && c.Rank == rank {
			total++
			if c.Discarded {
				discarded++
			}
		}
	}

	return total, discarded
}

func (g *Game) GetNotesSize() int {
	// Local variables
	variant := variants[g.Options.VariantName]

	// There are notes for every card in the deck + the stack bases for each suit
	numCards := len(g.Deck)
	numSuits := len(variant.Suits)
	return numCards + numSuits
}

*/