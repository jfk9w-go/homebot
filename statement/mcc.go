package statement

import (
	"strings"

	"gorm.io/gorm/clause"

	"github.com/jfk9w-go/flu/csv"
)

type MerchantCategoryCode struct {
	Code                string `gorm:"column:code;uniqueIndex:mcc_idx"`
	EditedDescription   string `gorm:"column:edited_description"`
	CombinedDescription string `gorm:"column:combined_description"`
	USDADescription     string `gorm:"column:usda_description"`
	IRSDescription      string `gorm:"column:irs_description"`
	IRSReportable       bool   `gorm:"column:irs_reportable"`
}

func (c MerchantCategoryCode) TableName() string {
	return "mcc"
}

type MerchantCategoryCodeDictionary map[string]MerchantCategoryCode

func (d MerchantCategoryCodeDictionary) Output(row *csv.Row) error {
	mcc := MerchantCategoryCode{
		Code:                strings.Trim(row.String("mcc"), " "),
		EditedDescription:   strings.Trim(row.String("edited_description"), " "),
		CombinedDescription: strings.Trim(row.String("combined_description"), " "),
		USDADescription:     strings.Trim(row.String("usda_description"), " "),
		IRSDescription:      strings.Trim(row.String("irs_description"), " "),
		IRSReportable:       strings.ToLower(strings.Trim(row.String("irs_reportable"), " ")) == "yes",
	}

	d[mcc.Code] = mcc
	return row.Err
}

func (d MerchantCategoryCodeDictionary) OnUpdate() []clause.Expression {
	return []clause.Expression{
		clause.OnConflict{
			Columns: []clause.Column{
				{Name: "code"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"edited_description", "combined_description",
				"usda_description", "irs_description",
				"irs_reportable",
			}),
		},
	}
}

func (d MerchantCategoryCodeDictionary) ForEach(iter Iterator) error {
	for _, mcc := range d {
		if err := iter(mcc); err != nil {
			return err
		}
	}

	return nil
}

func (d MerchantCategoryCodeDictionary) Cancel() interface{} {
	return nil
}

func (d MerchantCategoryCodeDictionary) Close() error {
	return nil
}
