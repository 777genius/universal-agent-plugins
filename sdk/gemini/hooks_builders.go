package gemini

func stopCommonResponse(reason string) CommonResponse {
	continueFalse := false
	return CommonResponse{Continue: &continueFalse, StopReason: reason}
}
