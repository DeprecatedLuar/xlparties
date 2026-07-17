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

// /vc_allow, /vc_deny
const (
	FailedResolveChannel = "failed to resolve the current channel"
	FailedLookupParty    = "failed to look up this party"
	MustBeOwner          = "Sorry blud. You must be the owner of the party channel. Currently that's <@%d>"
	FailedOverrideUser   = "failed to %s user"
	UserAllowed          = "<@%d> is now allowed in this party"
	UserDenied           = "<@%d> is now exiled from this party. (mwahaha)"
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

// /block
const (
	FailedBlockUser = "Errm... it seems *I* failed to block the user (please panic)"
	UserBlocked     = "blocked <@%d> (mwahaha)"
)

// /unblock
const (
	FailedUnblockUser = "Errm... it seems *I* failed to unblock user (please panic)"
	UserUnblocked     = "unblocked <@%d>"
)

// party ownership handoff notice, posted by internal/party.
const NewOwner = "Congratulations <@%d>! You have been elevated to the owner of this party."
