package main

import pb "github.com/azzykesuma/warframeMarket/api/proto"

func mapItemShort(item WfmItem) *pb.ItemShort {
	return &pb.ItemShort{
		Id:       item.Id,
		Slug:     item.Slug,
		GameRef:  item.GameRef,
		Tags:     item.Tags,
		Subtypes: item.Subtypes,
		Vaulted:  item.Vaulted,
		MaxRank:  item.MaxRank,
		Ducats:   item.Ducats,
		I18N:     mapI18n(item.I18n),
	}
}

func mapItemDetail(item WfmItem) *pb.ItemDetail {
	return &pb.ItemDetail{
		Id:         item.Id,
		Slug:       item.Slug,
		GameRef:    item.GameRef,
		Tags:       item.Tags,
		Subtypes:   item.Subtypes,
		Vaulted:    item.Vaulted,
		TradingTax: item.TradingTax,
		Tradable:   item.Tradable,
		MaxRank:    item.MaxRank,
		Ducats:     item.Ducats,
		I18N:       mapI18n(item.I18n),
	}
}

func mapI18n(i18n WfmI18n) *pb.ItemI18N {
	return &pb.ItemI18N{
		En: &pb.ItemLanguageDetail{
			Name:        i18n.En.Name,
			Description: i18n.En.Description,
			Icon:        i18n.En.Icon,
			Thumb:       i18n.En.Thumb,
			SubIcon:     i18n.En.SubIcon,
		},
	}
}

func mapOrders(items []WfmOrderListing) []*pb.Order {
	orders := make([]*pb.Order, 0, len(items))
	for _, item := range items {
		order := normalizeOrder(item)
		orders = append(orders, &pb.Order{
			Id:        order.Id,
			ItemId:    order.ItemId,
			ItemSlug:  order.ItemSlug,
			Price:     order.Platinum,
			Quantity:  order.Quantity,
			OrderType: orderType(order),
			Subtype:   order.Subtype,
			Visible:   order.Visible,
			CreatedAt: order.CreatedAt,
			UpdatedAt: order.UpdatedAt,
			User:      mapUser(item.User),
		})
	}
	return orders
}

func normalizeOrder(listing WfmOrderListing) WfmOrder {
	if listing.Order.Id != "" || listing.Order.Platinum != 0 || listing.Order.Quantity != 0 {
		return listing.Order
	}
	return WfmOrder{
		Id:        listing.Id,
		ItemId:    listing.ItemId,
		ItemSlug:  listing.ItemSlug,
		Type:      listing.Type,
		OrderType: listing.OrderType,
		Platinum:  listing.Platinum,
		Quantity:  listing.Quantity,
		Subtype:   listing.Subtype,
		Visible:   listing.Visible,
		CreatedAt: listing.CreatedAt,
		UpdatedAt: listing.UpdatedAt,
	}
}

func orderType(order WfmOrder) string {
	if order.Type != "" {
		return order.Type
	}
	return order.OrderType
}

func mapUser(user WfmUser) *pb.User {
	return &pb.User{
		Id:         user.Id,
		IngameName: user.IngameName,
		Status:     user.Status,
		Platform:   user.Platform,
		Reputation: user.Reputation,
	}
}
