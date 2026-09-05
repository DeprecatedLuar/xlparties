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

// /party_limit
const (
	MissingLimitOption  = "you must provide a limit"
	FailedSetPartyLimit = "failed to set user limit to %d"
	PartyLimitSet       = "this party's user limit is now **%d**"
	PartyLimitCleared   = "this party's user limit has been cleared - unlimited"
)

// /party_preset
const (
	PartyPresetPrompt    = "pick your saved default access mode (applies only when you next create a party)"
	PartyPresetSet       = "your default access mode is now **%s**"
	PartyPresetCleared   = "your saved default access mode has been cleared - new parties will use the default (%s)"
	FailedSetPartyPreset = "failed to set your default access mode"
	FailedClearPreset    = "failed to clear your saved default access mode"
	PartyPresetCurrent   = "**Your saved preset:** %s"
	NoPartyPreset        = "**Your saved preset:** _none (uses default: %s)_"

	FailedSetPartyPresetLimit = "failed to set your default user limit"
	PartyPresetLimitSet       = "your default user limit is now **%d**"
	PartyPresetLimitCleared   = "your default user limit has been cleared - unlimited"
	PartyPresetLimitCurrent   = "**Your saved user limit:** %d"
	NoPartyPresetLimit        = "**Your saved user limit:** _unlimited_"
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
	FailedAddFriend         = "failed to add friend"
	AlreadyFriend           = "<@%d> seems to already be your acquaintance"
	FriendAdded             = "Now you and <@%d> shall be besties. Yipee"
	FriendAddedStillBlocked = "<@%d> is on your friend list now, but you still have them blocked - they remain locked out until you `/enemy_remove` them too"
	FriendAddedNotif        = " ## %s\nIt seems <@%d> added you as a friend in **%s**.\nI would never do that _but_ you can use `/friend_add` (in the server) and pick <@%d> as the user to add them back"
)

// /friend_remove
const (
	FailedRemoveFriend = "Errm... it seems *I* failed to remove your friend (please panic)"
	FriendRemoved      = "<@%d> has been REMOVED as a friend (mwahaha)"
)

// /enemy_add
const (
	FailedAddEnemy        = "Errm... it seems *I* failed to add the enemy (please panic)"
	EnemyAdded            = "<@%d> is now your ENEMY and won't be able to join your parties any longer (as long as you're the owner)"
	EnemyAddedStillFriend = "<@%d> is now blocked, but they're still on your friend list too - a frenemy. They stay locked out until you `/friend_remove` them or `/enemy_remove` the block"
)

// /enemy_remove
const (
	FailedRemoveEnemy = "Errm... it seems *I* failed to remove the enemy (please panic)"
	EnemyRemoved      = "<@%d> is no longer your enemy"
)

// /relationships
const (
	FailedListRelationships = "failed to list your relationships"
	NoRelationships         = "you have no friends or enemies yet"
	FriendListHeader        = "**Your Friends**\n%s"
	EnemyListHeader         = "**Your Enemies**\n%s"
	FrenemyListHeader       = "**Your FRENEMIES**\n%s"
)

// /party_info
const (
	PartyInfoHeader  = "**Party type:** %s\n**User limit:** %s\n\n**Allowed in:**\n%s\n\n**Blocked:**\n%s\n\n%s\n%s"
	NoOverrides      = "_none_"
	PartyInfoNoLimit = "unlimited"
)

// /party_invite
const (
	FailedInviteUser              = "failed to invite user"
	MustBeInPartyChannel          = "you must be connected to this party's voice channel to invite someone"
	PartyInviteSent               = "<@%d> has been invited to this party"
	PartyInviteAlreadyHasAccess   = "<@%d> already has access to this party — sent them a reminder instead"
	PartyInviteRefused            = "<@%d> can't be invited to this party"
	PartyInviteDMAlreadyHasAccess = "## %s\n<@%d> tried to invite you to their party in this server, but you already have access - hop into their party voice channel whenever you like."
	PartyInviteDMBody             = "## %s\n<@%d> invited you to their party in this server. Here's your one-click join link (works once, and only until you use it or it expires):\n%s"
)

// party ownership handoff notice, posted by internal/party.
const NewOwner = "Congratulations <@%d>! You have been elevated to the owner of this party."

// party creation notice, posted by internal/party.
const PartyCreated = "## Welcome aboard, Captain <@%d>.\nThis channel is your designated party venue, currently operating in **Friends of Friends** mode.\n\nBe advised of the following:\n* You have **%d friend(s)** who can automatically see and join this channel.\n* Access rights may be adjusted using `/party_mode` (limit the scope to **friends-only**, make it **invite-only** if you hate your friends, or throw the doors open with **public** mode; your enemies stay locked out either way).\n* To allow _other_ people in you can use `/party_allow`, or `/party_block` to prevent your evil enemies from joining.\n* Anyone currently in this channel can `/party_invite` someone else in, regardless of friend status.\n* Use `/party_info` to check your current access mode and overrides at a glance.\n* For additional instruction, refer to `/help`."

// party creation notice for the public-mode default, posted by
// internal/party in place of PartyCreated.
const PartyCreatedPublic = "## Welcome aboard, Captain <@%d>.\nThis channel is your designated party venue, currently operating in **Public** mode: anyone can see and join.\n\nBe advised of the following:\n* Access rights may be adjusted using `/party_mode` (limit the scope to **friends-only**, **friends-of-friends**, or **invite-only** if you'd rather curate who gets in).\n* Your globally-blocked enemies stay locked out regardless of mode.\n* To keep specific people out you can use `/party_block`, or `/party_allow` to grant someone access even under a stricter mode.\n* Anyone currently in this channel can `/party_invite` someone else in.\n* Use `/party_info` to check your current access mode and overrides at a glance.\n* For additional instruction, refer to `/help`."

// posted alongside PartyCreated when the owner has zero friends, since
// "Friends of Friends" mode is otherwise silently useless to them.
const PartyCreatedNoFriendsWarning = "No friends means nobody can see or join this party automatically. Use `/party_invite` to bring someone in, or `/party_mode` to switch to public."

// appended as the last line of PartyCreated/PartyCreatedPublic when the
// owner has no saved /party_preset, since the command is otherwise easy to
// never discover.
const PartyPresetTip = "**Tip:** This party used the **default access mode** because you have no saved preset. Set one with `/party_preset` so future parties open the way you want."
