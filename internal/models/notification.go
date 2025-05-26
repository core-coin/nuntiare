package models

type NotificationProvider struct {
	// ID is the unique identifier for the notification provider.
	ID int64 `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	// Address is the address of the wallet.
	Address string `json:"address" gorm:"column:address;unique;not null"`
	// TelegramProvider is the telegram provider associated with the notification provider.
	TelegramProvider TelegramProvider `json:"telegram_provider" gorm:"foreignKey:NotificationProviderID;constraint:OnDelete:CASCADE"`
	// EmailProvider is the email provider associated with the notification provider.
	EmailProvider EmailProvider `json:"email_provider" gorm:"foreignKey:NotificationProviderID;constraint:OnDelete:CASCADE"`
}

type TelegramProvider struct {
	// ID is the unique identifier for the telegram provider.
	ID int64 `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	// NotificationProviderID is the foreign key to the NotificationProvider.
	NotificationProviderID int64 `json:"notification_provider_id" gorm:"column:notification_provider_id"`
	// Username is the username in the telegram.
	Username string `json:"username" gorm:"column:username;unique;not null"`
	// ChatID is the chat ID in the telegram.
	ChatID string `json:"chat_id" gorm:"column:chat_id;unique;not null"`
}

type EmailProvider struct {
	// ID is the unique identifier for the email provider.
	ID int64 `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	// NotificationProviderID is the foreign key to the NotificationProvider.
	NotificationProviderID int64 `json:"notification_provider_id" gorm:"column:notification_provider_id"`
	// Email is the email address of the user.
	Email string `json:"email" gorm:"column:email;unique;not null"`
}
