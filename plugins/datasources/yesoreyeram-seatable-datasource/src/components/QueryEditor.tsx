import React, { ChangeEvent, useCallback, useEffect, useState } from 'react';
import {
  InlineField,
  InlineFieldRow,
  Input,
  Select,
  MultiSelect,
  IconButton,
  RadioButtonGroup,
  Button,
  TextArea,
} from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';

import { DataSource } from '../datasource';
import { ColumnInfo, SeaTableDataSourceOptions, SeaTableQuery, SeaTableQueryType, TableInfo } from '../types';
import { parseSort, serializeSort, SortDirection, SortItem } from '../sort';
import { emptyRootGroup, FilterGroup, parseFilterTree, stringifyFilterTree } from '../filter';
import { FilterEditor } from './FilterEditor';

type Props = QueryEditorProps<DataSource, SeaTableQuery, SeaTableDataSourceOptions>;

const LABEL_WIDTH = 16;

const QUERY_TYPE_OPTIONS: Array<SelectableValue<SeaTableQueryType>> = [
  { label: 'Records', value: 'records', description: 'Return matching rows' },
  { label: 'Count', value: 'count', description: 'Return the number of matching rows' },
  { label: 'SQL', value: 'sql', description: 'Run a raw SeaTable SQL statement' },
];

const DIRECTION_OPTIONS: Array<SelectableValue<SortDirection>> = [
  { label: 'Asc', value: 'asc' },
  { label: 'Desc', value: 'desc' },
];

/**
 * Build the query patch for a table selection. When the table changes, the
 * table-dependent options (view, filters, sort, fields) are cleared because they
 * reference columns that no longer exist in the new table.
 */
export function tableChangePatch(tableName: string, changingTable: boolean): Partial<SeaTableQuery> {
  if (!changingTable) {
    return { tableName };
  }
  return { tableName, viewName: '', filterTree: '', sort: '', fields: '' };
}

export function QueryEditor({ query, onChange, onRunQuery, datasource }: Props) {
  const { tableName, viewName, sort, fields, filterTree, limit, sql } = query;
  const queryType = query.queryType ?? 'records';
  const isCount = queryType === 'count';
  const isSQL = queryType === 'sql';

  const [tables, setTables] = useState<TableInfo[]>([]);
  const [tableOptions, setTableOptions] = useState<Array<SelectableValue<string>>>([]);
  const [loadingTables, setLoadingTables] = useState(false);
  const [tablesError, setTablesError] = useState<string | undefined>();

  // Sort is persisted as a JSON array of {field, direction} but edited as rows.
  const [sortItems, setSortItems] = useState<SortItem[]>(() => parseSort(sort));

  useEffect(() => {
    if (serializeSort(sortItems) !== (sort ?? '')) {
      setSortItems(parseSort(sort));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sort]);

  // Structured filter tree edited via the filter builder. Persisted as JSON
  // (filterTree); the SQL is built server-side.
  const [filterRoot, setFilterRoot] = useState<FilterGroup>(() => parseFilterTree(filterTree));

  useEffect(() => {
    if (stringifyFilterTree(filterRoot) !== (filterTree ?? '')) {
      setFilterRoot(parseFilterTree(filterTree));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filterTree]);

  // Load the base's tables (and their columns) once.
  useEffect(() => {
    let cancelled = false;
    if (isSQL) {
      return;
    }
    setLoadingTables(true);
    setTablesError(undefined);
    datasource
      .getTables()
      .then((res) => {
        if (!cancelled) {
          setTables(res);
          setTableOptions(res.map((t) => ({ label: t.name, value: t.name })));
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setTablesError(err?.data?.error ?? err?.message ?? 'Failed to load tables');
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoadingTables(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [datasource, isSQL]);

  // Columns of the selected table, for the fields/filter/sort pickers.
  const currentColumns: ColumnInfo[] = tableName ? (tables.find((t) => t.name === tableName)?.columns ?? []) : [];
  const columnOptions: Array<SelectableValue<string>> = currentColumns.map((c) => ({
    label: c.name,
    value: c.name,
    description: c.type,
  }));

  const update = useCallback(
    (patch: Partial<SeaTableQuery>) => {
      onChange({ ...query, ...patch });
    },
    [onChange, query]
  );

  const onQueryTypeChange = (value: SeaTableQueryType) => {
    update({ queryType: value });
    onRunQuery();
  };

  const onTableSelect = (value: SelectableValue<string> | null) => {
    const name = value?.value ?? '';
    const changingTable = name !== tableName;
    if (changingTable) {
      setFilterRoot(emptyRootGroup());
      setSortItems([]);
    }
    update(tableChangePatch(name, changingTable));
    onRunQuery();
  };

  const selectedTable: SelectableValue<string> | null = tableName
    ? (tableOptions.find((t) => t.value === tableName) ?? { label: tableName, value: tableName })
    : null;

  const onViewChange = (e: ChangeEvent<HTMLInputElement>) => {
    update({ viewName: e.target.value });
  };

  const onLimitChange = (e: ChangeEvent<HTMLInputElement>) => {
    const n = parseInt(e.target.value, 10);
    update({ limit: isNaN(n) ? 0 : n });
  };

  const onSQLChange = (e: ChangeEvent<HTMLTextAreaElement>) => {
    update({ sql: e.target.value });
  };

  const onFilterChange = (root: FilterGroup) => {
    setFilterRoot(root);
    update({ filterTree: stringifyFilterTree(root) });
    onRunQuery();
  };

  const applySort = (items: SortItem[], run: boolean) => {
    setSortItems(items);
    const serialized = serializeSort(items);
    if (serialized !== (sort ?? '')) {
      update({ sort: serialized });
      if (run) {
        onRunQuery();
      }
    }
  };

  const onAddSort = () => {
    setSortItems([...sortItems, { field: '', direction: 'asc' }]);
  };

  const onRemoveSort = (index: number) => {
    applySort(
      sortItems.filter((_, i) => i !== index),
      true
    );
  };

  const onSortFieldChange = (index: number, value: SelectableValue<string> | null) => {
    const next = sortItems.map((item, i) => (i === index ? { ...item, field: value?.value ?? '' } : item));
    applySort(next, true);
  };

  const onSortDirectionChange = (index: number, direction: SortDirection) => {
    const next = sortItems.map((item, i) => (i === index ? { ...item, direction } : item));
    applySort(next, true);
  };

  // `fields` is persisted as a comma-separated string of column names.
  const selectedFields: Array<SelectableValue<string>> = (fields ?? '')
    .split(',')
    .map((f) => f.trim())
    .filter((f) => f.length > 0)
    .map((f) => columnOptions.find((o) => o.value === f) ?? { label: f, value: f });

  const onFieldsSelect = (values: Array<SelectableValue<string>>) => {
    const list = values.map((v) => v.value).filter((v): v is string => !!v);
    update({ fields: list.join(',') });
    onRunQuery();
  };

  return (
    <div>
      <div className="gf-form">
        <InlineField
          label="Query type"
          labelWidth={LABEL_WIDTH}
          tooltip="Records returns rows; Count returns the number of matching rows; SQL runs a raw SeaTable SQL statement."
        >
          <RadioButtonGroup<SeaTableQueryType>
            options={QUERY_TYPE_OPTIONS}
            value={queryType}
            onChange={onQueryTypeChange}
          />
        </InlineField>
      </div>

      {isSQL ? (
        <div className="gf-form" style={{ alignItems: 'flex-start' }}>
          <InlineField
            label="SQL"
            labelWidth={LABEL_WIDTH}
            tooltip="A SeaTable SQL statement, e.g. SELECT * FROM `Table1` WHERE Age > 21 ORDER BY Name LIMIT 100. Use ? placeholders with template variables for safe interpolation."
            grow
          >
            <TextArea
              rows={6}
              value={sql ?? ''}
              placeholder="SELECT * FROM `Table1` LIMIT 100"
              onChange={onSQLChange}
              onBlur={onRunQuery}
            />
          </InlineField>
        </div>
      ) : (
        <>
          <InlineFieldRow>
            <InlineField
              label="Table"
              labelWidth={LABEL_WIDTH}
              tooltip="Select a table within the base. The list is fetched from the SeaTable metadata API."
              error={tablesError}
              invalid={!!tablesError}
              required
            >
              <Select<string>
                width={40}
                isClearable
                isLoading={loadingTables}
                options={tableOptions}
                value={selectedTable}
                onChange={onTableSelect}
                allowCustomValue
                placeholder="Select table"
                noOptionsMessage="No tables found"
              />
            </InlineField>

            {!isCount && (
              <InlineField
                label="View"
                tooltip="Optional. Query a specific view of the table. A view is only applied for plain listings (no filters, sort or fields); those run via SQL, which has no view concept."
              >
                <Input
                  width={30}
                  value={viewName ?? ''}
                  placeholder={tableName ? 'Default view' : 'Select a table first'}
                  disabled={!tableName}
                  onChange={onViewChange}
                  onBlur={onRunQuery}
                />
              </InlineField>
            )}
          </InlineFieldRow>

          {!isCount && (
            <div className="gf-form">
              <InlineField
                label="Fields"
                labelWidth={LABEL_WIDTH}
                tooltip="Columns to return. Leave empty to return all columns. Selecting fields runs the query via SQL."
              >
                <MultiSelect<string>
                  width={40}
                  options={columnOptions}
                  value={selectedFields}
                  onChange={onFieldsSelect}
                  allowCustomValue
                  placeholder={tableName ? 'All columns' : 'Select a table first'}
                  disabled={!tableName}
                  noOptionsMessage="No columns found"
                />
              </InlineField>
            </div>
          )}

          <div className="gf-form" style={{ alignItems: 'flex-start' }}>
            <InlineField
              label="Filters"
              labelWidth={LABEL_WIDTH}
              tooltip="Filter rows. Add individual filters or nested filter groups. Compiled into a parameterized SQL WHERE clause server-side. Operators adapt to each column's type."
            >
              <FilterEditor group={filterRoot} columns={currentColumns} disabled={!tableName} onChange={onFilterChange} />
            </InlineField>
          </div>

          {!isCount && (
            <>
              <div className="gf-form" style={{ alignItems: 'flex-start' }}>
                <InlineField
                  label="Sort"
                  labelWidth={LABEL_WIDTH}
                  tooltip="Order results by one or more columns. Sorting runs the query via SQL."
                >
                  <div>
                    {sortItems.map((item, index) => (
                      <InlineFieldRow key={index} style={{ marginBottom: 4, alignItems: 'center' }}>
                        <Select<string>
                          width={28}
                          options={columnOptions}
                          value={
                            item.field
                              ? (columnOptions.find((o) => o.value === item.field) ?? {
                                  label: item.field,
                                  value: item.field,
                                })
                              : null
                          }
                          onChange={(v) => onSortFieldChange(index, v)}
                          allowCustomValue
                          placeholder="Select column"
                          disabled={!tableName}
                          noOptionsMessage="No columns found"
                        />
                        <RadioButtonGroup<SortDirection>
                          options={DIRECTION_OPTIONS}
                          value={item.direction}
                          onChange={(v) => onSortDirectionChange(index, v)}
                        />
                        <IconButton
                          name="trash-alt"
                          tooltip="Remove sort"
                          aria-label="Remove sort"
                          onClick={() => onRemoveSort(index)}
                        />
                      </InlineFieldRow>
                    ))}
                    <Button variant="secondary" size="sm" icon="plus" onClick={onAddSort} disabled={!tableName}>
                      Add sort
                    </Button>
                  </div>
                </InlineField>
              </div>

              <div className="gf-form">
                <InlineField
                  label="Limit"
                  labelWidth={LABEL_WIDTH}
                  tooltip="Maximum number of rows. 0 returns all (auto-paginated: 1000 rows/request via the rows endpoint, 10000 via SQL)."
                >
                  <Input
                    width={20}
                    type="number"
                    min={0}
                    value={limit ?? 0}
                    onChange={onLimitChange}
                    onBlur={onRunQuery}
                  />
                </InlineField>
              </div>
            </>
          )}
        </>
      )}
    </div>
  );
}
