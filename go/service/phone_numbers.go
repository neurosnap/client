package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/keybase/client/go/gregor"
	"github.com/keybase/client/go/libkb"
	"github.com/keybase/client/go/phonenumbers"
	gregor1 "github.com/keybase/client/go/protocol/gregor1"
	keybase1 "github.com/keybase/client/go/protocol/keybase1"
	"github.com/keybase/go-framed-msgpack-rpc/rpc"

	"golang.org/x/net/context"
)

type PhoneNumbersHandler struct {
	libkb.Contextified
	*BaseHandler
}

func NewPhoneNumbersHandler(xp rpc.Transporter, g *libkb.GlobalContext) *PhoneNumbersHandler {
	handler := &PhoneNumbersHandler{
		Contextified: libkb.NewContextified(g),
		BaseHandler:  NewBaseHandler(g, xp),
	}
	return handler
}

var _ keybase1.PhoneNumbersInterface = (*PhoneNumbersHandler)(nil)

func (h *PhoneNumbersHandler) AddPhoneNumber(ctx context.Context, arg keybase1.AddPhoneNumberArg) (err error) {
	mctx := libkb.NewMetaContext(ctx, h.G())
	defer mctx.TraceTimed("PhoneNumbersHandler#AddPhoneNumber", func() error { return err })()
	if err = libkb.IsPossiblePhoneNumber(arg.PhoneNumber); err != nil {
		return err
	}
	return phonenumbers.AddPhoneNumber(mctx, arg.PhoneNumber, arg.Visibility)
}

func (h *PhoneNumbersHandler) EditPhoneNumber(ctx context.Context, arg keybase1.EditPhoneNumberArg) (err error) {
	mctx := libkb.NewMetaContext(ctx, h.G())
	defer mctx.TraceTimed("PhoneNumbersHandler#AddPhoneNumber", func() error { return err })()
	if err = libkb.IsPossiblePhoneNumber(arg.OldPhoneNumber); err != nil {
		return err
	}
	if err = libkb.IsPossiblePhoneNumber(arg.PhoneNumber); err != nil {
		return err
	}

	if err = phonenumbers.DeletePhoneNumber(mctx, arg.OldPhoneNumber); err != nil {
		return err
	}
	return phonenumbers.AddPhoneNumber(mctx, arg.PhoneNumber, arg.Visibility)
}

func (h *PhoneNumbersHandler) VerifyPhoneNumber(ctx context.Context, arg keybase1.VerifyPhoneNumberArg) (err error) {
	mctx := libkb.NewMetaContext(ctx, h.G())
	defer mctx.TraceTimed("PhoneNumbersHandler#VerifyPhoneNumber", func() error { return err })()
	if err = libkb.IsPossiblePhoneNumber(arg.PhoneNumber); err != nil {
		return err
	}

	return phonenumbers.VerifyPhoneNumber(mctx, arg.PhoneNumber, arg.Code)
}

func (h *PhoneNumbersHandler) GetPhoneNumbers(ctx context.Context, sessionID int) (ret []keybase1.UserPhoneNumber, err error) {
	mctx := libkb.NewMetaContext(ctx, h.G())
	defer mctx.TraceTimed("PhoneNumbersHandler#GetPhoneNumbers", func() error { return err })()
	return phonenumbers.GetPhoneNumbers(mctx)
}

func (h *PhoneNumbersHandler) DeletePhoneNumber(ctx context.Context, arg keybase1.DeletePhoneNumberArg) (err error) {
	mctx := libkb.NewMetaContext(ctx, h.G())
	defer mctx.TraceTimed("PhoneNumbersHandler#DeletePhoneNumber", func() error { return err })()
	if err = libkb.IsPossiblePhoneNumber(arg.PhoneNumber); err != nil {
		return err
	}

	return phonenumbers.DeletePhoneNumber(mctx, arg.PhoneNumber)
}

func (h *PhoneNumbersHandler) SetVisibilityPhoneNumber(ctx context.Context, arg keybase1.SetVisibilityPhoneNumberArg) (err error) {
	mctx := libkb.NewMetaContext(ctx, h.G())
	defer mctx.TraceTimed("PhoneNumbersHandler#SetVisibilityPhoneNumber", func() error { return err })()
	if err = libkb.IsPossiblePhoneNumber(arg.PhoneNumber); err != nil {
		return err
	}

	return phonenumbers.SetVisibilityPhoneNumber(mctx, arg.PhoneNumber, arg.Visibility)
}

func (h *PhoneNumbersHandler) SetVisibilityAllPhoneNumber(ctx context.Context, arg keybase1.SetVisibilityAllPhoneNumberArg) (err error) {
	mctx := libkb.NewMetaContext(ctx, h.G())
	defer mctx.TraceTimed("PhoneNumbersHandler#SetVisibilityAllPhoneNumber", func() error { return err })()
	return phonenumbers.SetVisibilityAllPhoneNumber(mctx, arg.Visibility)
}

func (h *PhoneNumbersHandler) BulkLookupPhoneNumbers(ctx context.Context, arg keybase1.BulkLookupPhoneNumbersArg) ([]keybase1.PhoneNumberLookupResult, error) {
	var err error
	mctx := libkb.NewMetaContext(ctx, h.G())
	defer mctx.TraceTimed("PhoneNumbersHandler#BulkLookupPhoneNumbers", func() error { return err })()
	return phonenumbers.BulkLookupPhoneNumbers(mctx, arg.PhoneNumberContacts, arg.RegionCodes, arg.UserRegionCode)
}

const phoneNumbersGregorHandlerName = "phoneHandler"

type phoneNumbersGregorHandler struct {
	libkb.Contextified
}

var _ libkb.GregorInBandMessageHandler = (*phoneNumbersGregorHandler)(nil)

func newPhoneNumbersGregorHandler(g *libkb.GlobalContext) *phoneNumbersGregorHandler {
	return &phoneNumbersGregorHandler{
		Contextified: libkb.NewContextified(g),
	}
}

func (r *phoneNumbersGregorHandler) Create(ctx context.Context, cli gregor1.IncomingInterface, category string, item gregor.Item) (bool, error) {
	switch category {
	case "phone.added", "phone.verified", "phone.superseded":
		return true, r.handlePhoneMsg(ctx, cli, category, item)
	default:
		if strings.HasPrefix(category, "phone.") {
			return false, fmt.Errorf("unknown phoneNumbersGregorHandler category: %q", category)
		}
		return false, nil
	}
}

func (r *phoneNumbersGregorHandler) Dismiss(ctx context.Context, cli gregor1.IncomingInterface, category string, item gregor.Item) (bool, error) {
	return false, nil
}

func (r *phoneNumbersGregorHandler) IsAlive() bool {
	return true
}

func (r *phoneNumbersGregorHandler) Name() string {
	return phoneNumbersGregorHandlerName
}

func (r *phoneNumbersGregorHandler) handlePhoneMsg(ctx context.Context, cli gregor1.IncomingInterface, category string, item gregor.Item) error {
	mctx := libkb.NewMetaContext(ctx, r.G())
	mctx.Debug("phoneNumbersGregorHandler: %s received", category)
	var phoneNumber keybase1.PhoneNumber
	switch category {
	case "phone.added":
		var msg keybase1.PhoneNumberAddedMsg
		if err := json.Unmarshal(item.Body().Bytes(), &msg); err != nil {
			mctx.Debug("error unmarshaling %s item: %s", category, err)
			return err
		}
		mctx.Debug("%s unmarshaled: %+v", category, msg)
		phoneNumber = msg.PhoneNumber
	case "phone.verified":
		var msg keybase1.PhoneNumberVerifiedMsg
		if err := json.Unmarshal(item.Body().Bytes(), &msg); err != nil {
			mctx.Debug("error unmarshaling %s item: %s", category, err)
			return err
		}
		mctx.Debug("%s unmarshaled: %+v", category, msg)
		phoneNumber = msg.PhoneNumber
	case "phone.superseded":
		var msg keybase1.PhoneNumberSupersededMsg
		if err := json.Unmarshal(item.Body().Bytes(), &msg); err != nil {
			mctx.Debug("error unmarshaling %s item: %s", category, err)
			return err
		}
		mctx.Debug("%s unmarshaled: %+v", category, msg)
		phoneNumber = msg.PhoneNumber
	}

	phoneNumbers, err := phonenumbers.GetPhoneNumbers(mctx)
	if err != nil {
		mctx.Error("Could not get current phone number list during handlePhoneMsg: %s", err)
	} else {
		r.G().NotifyRouter.HandlePhoneNumbersChanged(ctx, phoneNumbers, category, phoneNumber)
	}
	return r.G().GregorState.DismissItem(ctx, cli, item.Metadata().MsgID())
}
