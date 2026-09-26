package auth

type OIDCLinkRequestDto struct {
	ReturnTo string `json:"returnTo"`
	Password string `json:"password"`
}

type OIDCUnlinkRequestDto struct {
	Password string `json:"password"`
}

type OIDCExchangeRequestDto struct {
	Grant string `json:"grant"`
}
