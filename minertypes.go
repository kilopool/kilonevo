package main

const (
	// Indicates there is no connection to the pool server, either because there has yet to
	// be a successful login, or there are connectivity issues. For the latter case, the
	// miner will continue trying to connect.
	MINING_PAUSED_NO_CONNECTION = -2

	// Indicates the most recent login failed so there is no connection to the pool server. If the
	// login failure was due to bad credentials, prompt the user to log in with valid log in
	// parameters.  If the failure is due to no connectivity, retry pool login with some backoff
	// policy.
	MINING_PAUSED_NO_LOGIN = -7

	// Indicates miner is actively mining
	MINING_ACTIVE = 1

	// for PokeChannel stuff:
	HANDLED    = 1
	USE_CACHED = 2

	STATE_CHANGE_POKE = 1
	EXIT_LOOP_POKE    = 8
	UPDATE_STATS_POKE = 9

	OVERRIDE_MINE  = 1
	OVERRIDE_PAUSE = 2
)
