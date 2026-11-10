// The fixed id of the WINGS Account federation subscription, matching
// config.FederationSubscriptionID on the Go side. The account owns this subscription: it is
// added on sign-in and removed on sign-out, and the user must not add or delete it by hand.
export const FEDERATION_SUBSCRIPTION_ID = 'wingsvpn-federation';
export const FEDERATION_SUBSCRIPTION_TITLE = 'WINGS Account';
