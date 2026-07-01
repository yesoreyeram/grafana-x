import React, { ChangeEvent, useCallback, useEffect, useState } from 'react';
import { Button, IconButton, InlineField, InlineFieldRow, InlineSwitch, Input, MultiSelect, RadioButtonGroup, Select } from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';

import { DataSource } from '../datasource';
import { FieldInfo, TeableDataSourceOptions, TeableQuery, TeableQueryType } from '../types';
import { parseSort, serializeSort, SortDirection, SortItem } from '../sort';
import { emptyRootGroup, FilterGroup, parseFilterTree, stringifyFilterTree } from '../filter';
import { FilterEditor } from './FilterEditor';

type Props = QueryEditorProps<DataSource, TeableQuery, TeableDataSourceOptions>;

const LABEL_WIDTH = 20;

const QUERY_TYPE_OPTIONS: Array<SelectableValue<TeableQueryType>> = [
  { label: 'Records', value: 'records', description: 'Return matching records' },
  { label: 'Count', value: 'count', description: 'Return the number of matching records' },
];

const DIRECTION_OPTIONS: Array<SelectableValue<SortDirection>> = [
  { label: 'Asc', value: 'asc' },
  { label: 'Desc', value: 'desc' },
];

export function QueryEditor({ query, onChange, onRunQuery, datasource }: Props) {
  const { baseId, tableId, fields, sort, filterTree, limit } = query;
  const queryType = query.queryType ?? 'records';
  const isCount = queryType === 'count';
  // A base id is only needed to list tables; fall back to the configured default.
  const effectiveBaseId = baseId || datasource.defaultBaseId;

  const [tableOptions, setTableOptions] = useState<Array<SelectableValue<string>>>([]);
  const [loadingTables, setLoadingTables] = useState(false);
  const [tablesError, setTablesError] = useState<string | undefined>();

  const [allFields, setAllFields] = useState<FieldInfo[]>([]);
  const [fieldOptions, setFieldOptions] = useState<Array<SelectableValue<string>>>([]);
  const [loadingFields, setLoadingFields] = useState(false);
  const [fieldsError, setFieldsError] = useState<string | undefined>();

  const [sortItems, setSortItems] = useState<SortItem[]>(() => parseSort(sort));

  useEffect(() => {
    if (serializeSort(sortItems) !== (sort ?? '')) {
      setSortItems(parseSort(sort));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sort]);

  const [filterRoot, setFilterRoot] = useState<FilterGroup>(() => parseFilterTree(filterTree));

  useEffect(() => {
    if (stringifyFilterTree(filterRoot) !== (filterTree ?? '')) {
      setFilterRoot(parseFilterTree(filterTree));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filterTree]);

  // Load tables when the (effective) base id changes
  useEffect(() => {
    let cancelled = false;
    if (!effectiveBaseId) {
      setTableOptions([]);
      setAllFields([]);
      setFieldOptions([]);
      setTablesError(undefined);
      setFieldsError(undefined);
      return;
    }
    setLoadingTables(true);
    setTablesError(undefined);
    datasource
      .getTables(effectiveBaseId)
      .then((res) => {
        if (!cancelled) {
          setTableOptions(res.map((t) => ({ label: t.name, value: t.id })));
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
  }, [effectiveBaseId, datasource]);

  // Load fields when tableId changes
  useEffect(() => {
    let cancelled = false;
    if (!tableId) {
      setAllFields([]);
      setFieldOptions([]);
      setFieldsError(undefined);
      return;
    }
    setLoadingFields(true);
    setFieldsError(undefined);
    datasource
      .getFields(tableId)
      .then((res) => {
        if (!cancelled) {
          setAllFields(res);
          setFieldOptions(res.map((f) => ({ label: f.name, value: f.name, description: f.type })));
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setFieldsError(err?.data?.error ?? err?.message ?? 'Failed to load fields');
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoadingFields(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [tableId, datasource]);

  const update = useCallback(
    (patch: Partial<TeableQuery>) => {
      onChange({ ...query, ...patch });
    },
    [onChange, query]
  );

  const onQueryTypeChange = (value: TeableQueryType) => {
    update({ queryType: value });
    onRunQuery();
  };

  const onBaseIdChange = (e: ChangeEvent<HTMLInputElement>) => {
    const id = e.target.value;
    const changingBase = id !== baseId;
    if (changingBase) {
      setFilterRoot(emptyRootGroup());
      setSortItems([]);
    }
    update({ baseId: id, tableId: changingBase ? '' : tableId, fields: changingBase ? '' : fields });
  };

  const onTableSelect = (value: SelectableValue<string> | null) => {
    const id = value?.value ?? '';
    update({ tableId: id, fields: '' });
    setFilterRoot(emptyRootGroup());
    setSortItems([]);
    onRunQuery();
  };

  const selectedTable: SelectableValue<string> | null = tableId
    ? (tableOptions.find((t) => t.value === tableId) ?? { label: tableId, value: tableId })
    : null;

  const onLimitChange = (e: ChangeEvent<HTMLInputElement>) => {
    const n = parseInt(e.target.value, 10);
    update({ limit: isNaN(n) ? 0 : n });
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

  const selectedFields: Array<SelectableValue<string>> = (fields ?? '')
    .split(',')
    .map((f) => f.trim())
    .filter((f) => f.length > 0)
    .map((f) => fieldOptions.find((o) => o.value === f) ?? { label: f, value: f });

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
          tooltip="Records returns rows; Count returns the number of matching records (respecting filters)."
        >
          <RadioButtonGroup<TeableQueryType>
            options={QUERY_TYPE_OPTIONS}
            value={queryType}
            onChange={onQueryTypeChange}
          />
        </InlineField>
      </div>

      <div className="gf-form">
        <InlineField
          label="Base ID"
          labelWidth={LABEL_WIDTH}
          tooltip="Teable base ID, used to list tables. Optional when a default base ID is configured on the data source."
          required={!datasource.defaultBaseId}
        >
          <Input
            width={40}
            name="baseId"
            placeholder={datasource.defaultBaseId || 'xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx'}
            value={baseId ?? ''}
            onChange={onBaseIdChange}
            onBlur={onRunQuery}
          />
        </InlineField>
      </div>

      <div className="gf-form">
        <InlineField
          label="Table"
          labelWidth={LABEL_WIDTH}
          tooltip="Select a table within the base."
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
            placeholder={effectiveBaseId ? 'Select table' : 'Enter a base ID first'}
            disabled={!effectiveBaseId}
            noOptionsMessage="No tables found"
          />
        </InlineField>
      </div>

      {!isCount && (
        <div className="gf-form">
          <InlineField
            label="Fields"
            labelWidth={LABEL_WIDTH}
            tooltip="Fields to return. Leave empty to return all fields."
            error={fieldsError}
            invalid={!!fieldsError}
          >
            <MultiSelect<string>
              width={40}
              isLoading={loadingFields}
              options={fieldOptions}
              value={selectedFields}
              onChange={onFieldsSelect}
              allowCustomValue
              placeholder={tableId ? 'All fields' : 'Select a table first'}
              disabled={!tableId}
              noOptionsMessage="No fields found"
            />
          </InlineField>
        </div>
      )}

      <div className="gf-form" style={{ alignItems: 'flex-start' }}>
        <InlineField
          label="Filters"
          labelWidth={LABEL_WIDTH}
          tooltip="Filter records. Compiled into a Teable JSON filter object server-side."
        >
          <FilterEditor group={filterRoot} fields={allFields} disabled={!tableId} onChange={onFilterChange} />
        </InlineField>
      </div>

      {!isCount && (
        <>
          <div className="gf-form" style={{ alignItems: 'flex-start' }}>
            <InlineField
              label="Sort"
              labelWidth={LABEL_WIDTH}
              tooltip="Order results by one or more fields."
            >
              <div>
                {sortItems.map((item, index) => (
                  <InlineFieldRow key={index} style={{ marginBottom: 4, alignItems: 'center' }}>
                    <Select<string>
                      width={28}
                      options={fieldOptions}
                      value={
                        item.field
                          ? (fieldOptions.find((o) => o.value === item.field) ?? { label: item.field, value: item.field })
                          : null
                      }
                      onChange={(v) => onSortFieldChange(index, v)}
                      allowCustomValue
                      placeholder="Select field"
                      disabled={!tableId}
                      noOptionsMessage="No fields"
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
                <Button variant="secondary" size="sm" icon="plus" onClick={onAddSort} disabled={!tableId}>
                  Add sort
                </Button>
              </div>
            </InlineField>
          </div>

          <InlineFieldRow>
            <InlineField
              label="Limit"
              labelWidth={LABEL_WIDTH}
              tooltip="Maximum number of records. 0 returns all (auto-paginated)."
            >
              <Input width={20} type="number" min={0} value={limit ?? 0} onChange={onLimitChange} onBlur={onRunQuery} />
            </InlineField>
          </InlineFieldRow>
        </>
      )}
      <InlineFieldRow>
        <InlineField
          label="Hide system fields"
          labelWidth={LABEL_WIDTH}
          tooltip="Hide metadata-style columns (id, created_at/updated_at, underscore-prefixed names, etc.) from the returned frame."
        >
          <InlineSwitch
            value={!!query.hideSystemFields}
            onChange={(e) => {
              update({ hideSystemFields: e.currentTarget.checked });
              onRunQuery();
            }}
          />
        </InlineField>
      </InlineFieldRow>

    </div>
  );
}
