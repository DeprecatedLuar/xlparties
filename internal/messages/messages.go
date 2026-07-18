// Package messages centralizes all user-facing Discord message text sent by
// the bot (command responses, DMs, channel notices), so wording can be
// audited or changed in one place without touching the logic that sends it.
package messages

// Shared across command handlers via callerAndTarget.
const (
	FailedResolveCaller = "failed to resolve your user id"
	FailedResolveTarget = "failed to resolve target user id"
	CannotTargetSelf    = "you cannot target yourself"
)

// /party_allow, /party_block
const (
	FailedResolveChannel = "failed to resolve the current channel"
	FailedLookupParty    = "failed to look up this party"
	NotInParty           = "My sibling in Lord... thou art not even in a party."
	MustBeOwner          = "Sorry blud. You must be the owner of the party channel. Currently that's <@%d>"
	FailedOverrideUser   = "failed to %s user"
	UserAllowed          = "<@%d> is now allowed in this party"
	UserDenied           = "<@%d> is now exiled from this party. (mwahaha)"
)

// /party_kick
const (
	FailedKickUser = "failed to kick user"
	UserKicked     = "<@%d> has been kicked from this party."
	UserNotPresent = "User <@%d> is not in this voice channel."
)

// /party_ban
const (
	FailedBanUser = "failed to ban user"
	UserBanned    = "<@%d> has been banned and kicked from this party."
)

// /party_mode
const (
	PartyModePrompt    = "pick an access mode for this party"
	PartyModeSet       = "this party's access mode is now **%s**"
	FailedSetPartyMode = "failed to set access mode to %s"
)

// /configure
const (
	ExpectedOneSubcommand = "I may have expected exactly one /configure subcommand"
	UnknownSubcommand     = "unknown /configure subcommand"
	FailedSaveWatchChan   = "failed to save watch channel"
	WatchChannelSet       = "watch channel set to <#%s>"
	FailedSaveCategory    = "failed to save category"
	CategorySet           = "party category set to <#%s>"
)

// /friend_add
const (
	FailedAddFriend  = "failed to add friend"
	AlreadyFriend    = "<@%d> seems to already be your acquaintance"
	FriendAdded      = "Now you and <@%d> shall be besties. Yipee"
	FriendAddedNotif = " ## Salutations Hooman.\nIt seems <@%d> added you as a friend in **%s**.\nI would never do that _but_ you can use `/friend_add` (in the server) and pick <@%d> as the user to add them back"
)

// /friend_remove
const (
	FailedRemoveFriend = "Errm... it seems *I* failed to remove your friend (please panic)"
	FriendRemoved      = "<@%d> has been REMOVED as a friend (mwahaha)"
)

// /enemy_add
const (
	FailedAddEnemy = "Errm... it seems *I* failed to add the enemy (please panic)"
	EnemyAdded     = "<@%d> is now your ENEMY and won't be able to join your parties any longer (as long as you're the owner)"
)

// /enemy_remove
const (
	FailedRemoveEnemy = "Errm... it seems *I* failed to remove the enemy (please panic)"
	EnemyRemoved      = "<@%d> is no longer your enemy"
)

// /friend_list, /enemy_list
const (
	FailedListFriends = "failed to list friends"
	FailedListEnemies = "failed to list enemies"
	NoFriends         = "you have no friends yet"
	NoEnemies         = "you have no enemies yet"
	FriendListHeader  = "**Your friends:**\n%s"
	EnemyListHeader   = "**Your enemies:**\n%s"
)

// party ownership handoff notice, posted by internal/party.
const NewOwner = "Congratulations <@%d>! You have been elevated to the owner of this party."

// party creation notice, posted by internal/party.
const PartyCreated = "## Salutations <@%d>.\nThis channel is your designated party venue, currently operating in \"Friends of Friends\" mode.\n\nBe advised of the following:\n* Your friends are permitted to see and join this channel automatically.\n* Access rights may be adjusted using `/party_mode` (you can limit the scope to friends-only _or_ make it invite-only if you hate your friends).\n* To allow _other_ people in you can use `/party_allow`, or `/party_block` to prevent your evil enemies from joining.\n* For additional instruction, refer to `/help`."

