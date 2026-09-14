// Package messages centralizes all user-facing Discord message text sent by
// the bot (command responses, DMs, channel notices), so wording can be
// audited or changed in one place without touching the logic that sends it.
package messages

import "xlparties/internal/store"

// Shared across command handlers via callerAndTarget.
const (
	FailedResolveCaller = "failed to resolve your user id"
	FailedResolveTarget = "failed to resolve target user id"
	CannotTargetSelf    = "you cannot target yourself"
	NuhUh               = "Nuh-uh"
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

// /user_friend
const (
	FailedAddFriend         = "failed to add friend"
	AlreadyFriend           = "<@%d> seems to already be your acquaintance"
	FriendAdded             = "You have now befriended <@%d>"
	FriendAddedStillBlocked = "<@%d> is on your friend list now, but you still have them blocked - they remain locked out until you `/user_unblock` them too"
	FriendAddedNotif        = " ## %s\nIt seems <@%d> added you as a friend in **%s**.\nI would never do that _but_ you can use `/user_friend` (in the server) and pick <@%d> as the user to add them back"
)

// /user_unfriend
const (
	FailedRemoveFriend = "Errm... it seems *I* failed to remove your friend (please panic)"
	FriendRemoved      = "<@%d> has been REMOVED as a friend (mwahaha)"
)

// /user_block
const (
	FailedAddEnemy        = "Errm... it seems *I* failed to add the enemy (please panic)"
	EnemyAdded            = "<@%d> is now your ENEMY and won't be able to join your parties any longer (as long as you're the owner)"
	EnemyAddedStillFriend = "<@%d> is now blocked, but they're still on your friend list too - a frenemy. They stay locked out until you `/user_unfriend` them or `/user_unblock` the block"
)

// /user_unblock
const (
	FailedRemoveEnemy = "Errm... it seems *I* failed to remove the enemy (please panic)"
	EnemyRemoved      = "<@%d> is no longer your enemy"
)

// /user_favorite
const (
	FailedAddFavorite         = "Errm... it seems *I* failed to add the bestie (please panic)"
	AlreadyFavorite           = "<@%d> seems to already be your bestie"
	FavoriteAdded             = "Now you and <@%d> shall be besties. Yipee (They are allowed on the `Besties Only` party mode)"
	FavoriteAddedStillBlocked = "<@%d> is your bestie now, but you still have them blocked - a best frenemy. They stay locked out until you `/user_unblock` them"
)

// /user_unfavorite
const (
	FailedRemoveFavorite = "Errm... it seems *I* failed to remove the bestie (please panic)"
	FavoriteRemoved      = "<@%d> is no longer your bestie, but they're still your friend"
)

// /user_list
const (
	FailedListRelationships = "failed to list your relationships"
	NoRelationships         = "you have no friends or enemies yet"
	BestieListHeader        = "**Your Besties**\n%s"
	FriendListHeader        = "**Your Friends**\n%s"
	EnemyListHeader         = "**Your Enemies**\n%s"
	FrenemyListHeader       = "**Your FRENEMIES**\n%s"
	BestFrenemyListHeader   = "**Your BEST FRENEMIES**\n%s"
)

// /party_info
const (
	PartyInfoHeader  = "**Party type:** %s\n**User limit:** %s\n\n**Allowed in:**\n%s\n\n**Blocked:**\n%s\n\n%s\n%s"
	NoOverrides      = "_none_"
	PartyInfoNoLimit = "unlimited"
	EveryoneMention  = "@everyone"
)

// /party_invite
const (
	FailedInviteUser     = "failed to invite user"
	MustBeInPartyChannel = "you must be connected to this party's voice channel to invite someone"
	PartyInviteSent      = "<@%d> has been invited to this party"
	PartyInviteRefused   = "<@%d> can't be invited to this party"
	// The trailing %d is a unix timestamp rendered by Discord's own
	// timestamp markup, so the reader sees it in their local timezone.
	PartyInviteDMBody = "## %s\n<@%d> invited you to their party in this server. Just click here to join (expires <t:%d:R>):\n%s"
)

// party ownership handoff notice, posted by internal/party.
const NewOwner = "Congratulations <@%d>! You have been elevated to the owner of this party."

// AccessModeLabel is the human-readable name for each store.AccessMode*
// constant, shared by every user-facing message that names a mode.
var AccessModeLabel = map[string]string{
	store.AccessModeFriendsOfFriends: "Friends of Friends",
	store.AccessModeFriendsOnly:      "Friends Only",
	store.AccessModeBestiesOnly:      "Besties Only",
	store.AccessModeInviteOnly:       "Invite Only",
	store.AccessModePublic:           "Public",
}

// party creation notice, posted by internal/party. %s is the mode's
// AccessModeLabel entry - PartyCreated covers every non-public mode.
const PartyCreated = "## Welcome aboard, Captain <@%d>.\nThis channel is your designated party venue, currently operating in **%s** mode.\n\nBe advised of the following:\n* You have **%d friend(s)** who can automatically see and join this channel.\n* Access rights may be adjusted using `/party_mode` (limit the scope to **friends-only**, narrow it further to **besties-only**, make it **invite-only** if you hate your friends, or throw the doors open with **public** mode; your enemies stay locked out either way).\n* To allow _other_ people in you can use `/party_allow`, or `/party_block` to prevent your evil enemies from joining.\n* Anyone currently in this channel can `/party_invite` someone else in, regardless of friend status.\n* Use `/party_info` to check your current access mode and overrides at a glance.\n* For additional instruction, refer to `/help`."

// party creation notice for the public-mode default, posted by
// internal/party in place of PartyCreated.
const PartyCreatedPublic = "## Welcome aboard, Captain <@%d>.\nThis channel is your designated party venue, currently operating in **Public** mode: anyone can see and join.\n\nBe advised of the following:\n* Access rights may be adjusted using `/party_mode` (limit the scope to **friends-only**, **friends-of-friends**, **besties-only**, or **invite-only** if you'd rather curate who gets in).\n* Your globally-blocked enemies stay locked out regardless of mode.\n* To keep specific people out you can use `/party_block`, or `/party_allow` to grant someone access even under a stricter mode.\n* Anyone currently in this channel can `/party_invite` someone else in.\n* Use `/party_info` to check your current access mode and overrides at a glance.\n* For additional instruction, refer to `/help`."

// posted alongside PartyCreated when the owner has zero friends, since
// "Friends of Friends" mode is otherwise silently useless to them.
const PartyCreatedNoFriendsWarning = "No friends means nobody can see or join this party automatically. Use `/party_invite` to bring someone in, or `/party_mode` to switch to public."

// party creation notice for besties_only mode, posted by internal/party in
// place of PartyCreated. %s is the mode's AccessModeLabel entry, %d the
// owner's bestie count.
const PartyCreatedBesties = "## Welcome aboard, Captain <@%d>.\nThis channel is your designated party venue, currently operating in **%s** mode.\n\nBe advised of the following:\n* You have **%d bestie(s)** who can automatically see and join this channel.\n* Access rights may be adjusted using `/party_mode` (widen the scope to **friends-only**, make it **invite-only** if you hate your besties, or throw the doors open with **public** mode; your enemies stay locked out either way).\n* To allow _other_ people in you can use `/party_allow`, or `/party_block` to prevent your evil enemies from joining.\n* Anyone currently in this channel can `/party_invite` someone else in, regardless of bestie status.\n* Use `/party_info` to check your current access mode and overrides at a glance.\n* For additional instruction, refer to `/help`."

// party creation notice for invite_only mode, posted by internal/party in
// place of PartyCreated. No count is shown since invite_only grants nobody
// automatic access, friends and besties included. %s is the mode's
// AccessModeLabel entry.
const PartyCreatedInviteOnly = "## Welcome aboard, Captain <@%d>.\nThis channel is your designated party venue, currently operating in **%s** mode: nobody gets in automatically, not even your friends or besties.\n\nBe advised of the following:\n* To let someone in, use `/party_allow`, or have anyone currently in the channel run `/party_invite`.\n* `/party_block` still keeps your evil enemies out, though they can't get in automatically here anyway.\n* Access rights may be adjusted using `/party_mode` if you'd rather open things up.\n* Use `/party_info` to check your current access mode and overrides at a glance.\n* For additional instruction, refer to `/help`."

// posted alongside PartyCreatedBesties when the owner has zero besties,
// since "Besties Only" mode is otherwise silently useless to them.
const PartyCreatedNoBestiesWarning = "No besties means nobody can see or join this party automatically. Use `/party_invite` to bring someone in, or `/party_mode` to switch to public."

// appended as the last line of PartyCreated/PartyCreatedPublic when the
// owner has no saved /party_preset, since the command is otherwise easy to
// never discover.
const PartyPresetTip = "**Tip:** This party used the **default access mode** because you have no saved preset. Set one with `/party_preset` so future parties open the way you want."

// party_create command responses, posted by internal/commands.
const (
	FailedCreateParty        = "failed to create your party"
	PartyCreateReady         = "Your party is ready: <#%d>. Hop in whenever you like."
	PartyCreateAlreadyExists = "You already have a party: <#%d>."
)
