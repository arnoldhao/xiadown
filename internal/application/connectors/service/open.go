package service

import (
	"context"
	"strings"

	"github.com/chromedp/chromedp"

	"xiadown/internal/application/browsercdp"
	"xiadown/internal/application/connectors/dto"
	"xiadown/internal/domain/connectors"
)

func (service *ConnectorsService) OpenConnectorSite(ctx context.Context, request dto.OpenConnectorSiteRequest) error {
	id := strings.TrimSpace(request.ID)
	if id == "" {
		return connectors.ErrInvalidConnector
	}
	connector, err := service.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	cookies := decodeCookies(connector.CookiesJSON)
	if len(cookies) == 0 {
		return connectors.ErrNoCookies
	}
	targetURL, err := connectorHomeURL(connector.Type)
	if err != nil {
		return err
	}
	userDataDir := connectorOpenDir(connector.Type, service.newSessionID())
	runtime, tabCtx, cancel, err := service.startBrowser(service.preferredBrowser(ctx), false, userDataDir)
	if err != nil {
		return err
	}
	defer cancel()
	defer runtime.Stop()
	defer func() {
		if service.removeAll != nil {
			_ = service.removeAll(userDataDir)
		}
	}()

	if err := chromedp.Run(tabCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		return browsercdp.SetCookies(ctx, targetURL, cookies)
	})); err != nil {
		return err
	}
	if err := chromedp.Run(tabCtx, chromedp.Navigate(targetURL)); err != nil {
		return err
	}
	_, err = waitForConnectorTabClose(ctx, runtime, tabCtx, connectorTargetIDFromContext(tabCtx), false, service.readCookies)
	return err
}
