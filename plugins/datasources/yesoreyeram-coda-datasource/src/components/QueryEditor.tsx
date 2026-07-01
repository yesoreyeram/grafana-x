import React, { ChangeEvent, useCallback, useEffect, useState } from 'react';
import { InlineField, InlineFieldRow, InlineSwitch, Input, Select, MultiSelect, RadioButtonGroup } from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';

import { DataSource } from '../datasource';
import {
  ColumnInfo,
  CodaDataSourceOptions,
  CodaQuery,
  CodaQueryType,
  CodaSortBy,
  CodaValueFormat,
} from '../types';

type Props = QueryEditorProps<DataSource, CodaQuery, CodaDataSourceOptions>;

const LABEL_WIDTH = 20;

const QUERY_TYPE_OPTIONS: Array<SelectableValue<CodaQueryType>> = [
  { label: 'Rows', value: 'rows', description: 'Return matching rows' },
  { label: 'Count', value: 'count', description: 'Return the number of rows' },
];

const SORT_BY_OPTIONS: Array<SelectableValue<CodaSortBy>> = [
  { label: 'Default', value: '', description: 'Coda default (creation order ascending)' },
  { label: 'Created', value: 'createdAt', description: 'Order by creation time' },
  { label: 'Updated', value: 'updatedAt', description: 'Order by last update time' },
  { label: 'Natural', value: 'natural', description: 'Table view order (implies visible only)' },
];

const VALUE_FORMAT_OPTIONS: Array<SelectableValue<CodaValueFormat>> = [
  { label: 'Simple', value: 'simple', description: 'Scalar values; arrays as comma strings' },
  { label: 'Arrays', value: 'simpleWithArrays', description: 'Array values kept as JSON arrays' },
  { label: 'Rich', value: 'rich', description: 'Lossless encoding (Markdown / JSON-LD)' },
];

export function QueryEditor({ query, onChange, onRunQuery, datasource }: Props) {
  const {
    tableId,
    columns,
    filterColumn,
    filterValue,
    query: rawQuery,
    sortBy,
    valueFormat,
    visibleOnly,
    limit,
    hideSystemFields,
  } = query;
  const queryType = query.queryType ?? 'rows';
  const isCount = queryType === 'count';
  const configuredDocId = datasource.docId;
  const showDocPicker = !configuredDocId;

  const selectedDocId = showDocPicker ? (query.docId ?? '') : configuredDocId;
  const docId = selectedDocId;

  const [docs, setDocs] = useState<Array<SelectableValue<string>>>([]);
  const [loadingDocs, setLoadingDocs] = useState(false);
  const [docsError, setDocsError] = useState<string | undefined>();

  const [tables, setTables] = useState<Array<SelectableValue<string>>>([]);
  const [loadingTables, setLoadingTables] = useState(false);
  const [tablesError, setTablesError] = useState<string | undefined>();

  const [columnOptions, setColumnOptions] = useState<Array<SelectableValue<string>>>([]);
  const [loadingColumns, setLoadingColumns] = useState(false);
  const [columnsError, setColumnsError] = useState<string | undefined>();

  useEffect(() => {
    if (!showDocPicker) {
      return;
    }
    let cancelled = false;
    setLoadingDocs(true);
    setDocsError(undefined);
    datasource
      .getDocs()
      .then((res) => {
        if (!cancelled) {
          setDocs(res.map((d) => ({ label: d.title, value: d.id, description: d.id })));
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setDocsError(err?.data?.error ?? err?.message ?? 'Failed to load docs');
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoadingDocs(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [showDocPicker, datasource]);

  useEffect(() => {
    let cancelled = false;
    if (showDocPicker && !docId) {
      setTables([]);
      setTablesError(undefined);
      return;
    }
    setLoadingTables(true);
    setTablesError(undefined);
    datasource
      .getTables(docId || undefined)
      .then((res) => {
        if (!cancelled) {
          setTables(res.map((t) => ({ label: t.title, value: t.id, description: t.id })));
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
  }, [docId, showDocPicker, datasource]);

  useEffect(() => {
    let cancelled = false;
    if (!tableId) {
      setColumnOptions([]);
      setColumnsError(undefined);
      return;
    }
    setLoadingColumns(true);
    setColumnsError(undefined);
    datasource
      .getColumns(tableId, docId || undefined)
      .then((res: ColumnInfo[]) => {
        if (!cancelled) {
          setColumnOptions(res.map((c) => ({ label: c.title, value: c.title, description: c.type })));
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setColumnsError(err?.data?.error ?? err?.message ?? 'Failed to load columns');
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoadingColumns(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [tableId, docId, datasource]);

  const update = useCallback(
    (patch: Partial<CodaQuery>) => {
      onChange({ ...query, ...patch });
    },
    [onChange, query]
  );

  const onQueryTypeChange = (value: CodaQueryType) => {
    update({ queryType: value });
    onRunQuery();
  };

  const onDocSelect = (value: SelectableValue<string> | null) => {
    const id = value?.value ?? '';
    if (id === selectedDocId) {
      return;
    }
    update({ docId: id, tableId: '', columns: '', filterColumn: '', filterValue: '', query: '' });
    onRunQuery();
  };

  const selectedDoc: SelectableValue<string> | null = selectedDocId
    ? (docs.find((d) => d.value === selectedDocId) ?? { label: selectedDocId, value: selectedDocId })
    : null;

  const onTableIdSelect = (value: SelectableValue<string> | null) => {
    const id = value?.value ?? '';
    update({ tableId: id, columns: '', filterColumn: '', filterValue: '', query: '' });
    onRunQuery();
  };

  const selectedTable: SelectableValue<string> | null = tableId
    ? (tables.find((t) => t.value === tableId) ?? { label: tableId, value: tableId })
    : null;

  const onLimitChange = (e: ChangeEvent<HTMLInputElement>) => {
    const n = parseInt(e.target.value, 10);
    update({ limit: isNaN(n) ? 0 : n });
  };

  const selectedColumns: Array<SelectableValue<string>> = (columns ?? '')
    .split(',')
    .map((f) => f.trim())
    .filter((f) => f.length > 0)
    .map((f) => columnOptions.find((o) => o.value === f) ?? { label: f, value: f });

  const onColumnsSelect = (values: Array<SelectableValue<string>>) => {
    const list = values.map((v) => v.value).filter((v): v is string => !!v);
    update({ columns: list.join(',') });
    onRunQuery();
  };

  const selectedFilterColumn: SelectableValue<string> | null = filterColumn
    ? (columnOptions.find((o) => o.value === filterColumn) ?? { label: filterColumn, value: filterColumn })
    : null;

  const onFilterColumnSelect = (value: SelectableValue<string> | null) => {
    update({ filterColumn: value?.value ?? '' });
    onRunQuery();
  };

  const onFilterValueChange = (e: ChangeEvent<HTMLInputElement>) => {
    update({ filterValue: e.target.value });
  };

  const onRawQueryChange = (e: ChangeEvent<HTMLInputElement>) => {
    update({ query: e.target.value });
  };

  return (
    <div>
      <div className="gf-form">
        <InlineField
          label="Query type"
          labelWidth={LABEL_WIDTH}
          tooltip="Rows returns matching rows; Count returns the number of rows (uses the table row count when unfiltered, otherwise counts matching rows)."
        >
          <RadioButtonGroup<CodaQueryType> options={QUERY_TYPE_OPTIONS} value={queryType} onChange={onQueryTypeChange} />
        </InlineField>
      </div>

      {showDocPicker && (
        <div className="gf-form">
          <InlineField
            label="Doc"
            labelWidth={LABEL_WIDTH}
            tooltip="Select a Coda doc. The list is fetched with your API token. You can also type a doc id manually."
            error={docsError}
            invalid={!!docsError}
            required
          >
            <Select<string>
              width={40}
              isClearable
              isLoading={loadingDocs}
              options={docs}
              value={selectedDoc}
              onChange={onDocSelect}
              allowCustomValue
              placeholder="Select doc"
              noOptionsMessage="No docs found"
            />
          </InlineField>
        </div>
      )}

      <InlineFieldRow>
        <InlineField
          label="Table"
          labelWidth={LABEL_WIDTH}
          tooltip="Select a Coda table. The list is fetched for the selected doc. You can also type a table id or name manually."
          error={tablesError}
          invalid={!!tablesError}
          required
        >
          <Select<string>
            width={40}
            isClearable
            isLoading={loadingTables}
            options={tables}
            value={selectedTable}
            onChange={onTableIdSelect}
            allowCustomValue
            disabled={showDocPicker && !docId}
            placeholder={showDocPicker && !docId ? 'Select a doc first' : 'Select table'}
            noOptionsMessage="No tables found"
          />
        </InlineField>
      </InlineFieldRow>

      {!isCount && (
        <div className="gf-form">
          <InlineField
            label="Hide system fields"
            labelWidth={LABEL_WIDTH}
            tooltip="Hide synthetic row-metadata columns (id, name, index, createdAt, updatedAt, href, browserLink) from the returned frame."
          >
            <InlineSwitch
              value={!!hideSystemFields}
              onChange={(e) => {
                update({ hideSystemFields: e.currentTarget.checked });
                onRunQuery();
              }}
            />
          </InlineField>
        </div>
      )}

      {!isCount && (
        <div className="gf-form">
          <InlineField
            label="Columns"
            labelWidth={LABEL_WIDTH}
            tooltip="Columns to return. Leave empty to return all columns. Coda always returns every column; the selection is applied server-side after fetching."
            error={columnsError}
            invalid={!!columnsError}
          >
            <MultiSelect<string>
              width={40}
              isLoading={loadingColumns}
              options={columnOptions}
              value={selectedColumns}
              onChange={onColumnsSelect}
              allowCustomValue
              placeholder={tableId ? 'All columns' : 'Select a table first'}
              disabled={!tableId}
              noOptionsMessage="No columns found"
            />
          </InlineField>
        </div>
      )}

      <InlineFieldRow>
        <InlineField
          label="Filter"
          labelWidth={LABEL_WIDTH}
          tooltip="Single-column equality filter, applied by the Coda API. Coda's rows endpoint can only filter by one column; use Grafana transformations for anything more complex."
        >
          <Select<string>
            width={24}
            isClearable
            isLoading={loadingColumns}
            options={columnOptions}
            value={selectedFilterColumn}
            onChange={onFilterColumnSelect}
            allowCustomValue
            disabled={!tableId}
            placeholder="Column"
            noOptionsMessage="No columns found"
          />
        </InlineField>
        <InlineField label="equals" labelWidth={10}>
          <Input
            width={24}
            value={filterValue ?? ''}
            placeholder="Value"
            onChange={onFilterValueChange}
            onBlur={onRunQuery}
            disabled={!tableId || !filterColumn}
          />
        </InlineField>
      </InlineFieldRow>

      <div className="gf-form">
        <InlineField
          label="Advanced query"
          labelWidth={LABEL_WIDTH}
          tooltip={'Raw Coda query, e.g. c-aBc123:"Apple" or "My Column":42. Takes precedence over the Filter above.'}
        >
          <Input
            width={40}
            value={rawQuery ?? ''}
            placeholder={'<column>:<value> (optional)'}
            onChange={onRawQueryChange}
            onBlur={onRunQuery}
            disabled={!tableId}
          />
        </InlineField>
      </div>

      {!isCount && (
        <InlineFieldRow>
          <InlineField label="Sort by" labelWidth={LABEL_WIDTH} tooltip="Row sort order. Natural matches the table view and implies visible-only rows.">
            <Select<CodaSortBy>
              width={20}
              options={SORT_BY_OPTIONS}
              value={SORT_BY_OPTIONS.find((o) => o.value === (sortBy ?? '')) ?? SORT_BY_OPTIONS[0]}
              onChange={(v) => {
                update({ sortBy: v?.value ?? '' });
                onRunQuery();
              }}
            />
          </InlineField>
          <InlineField label="Visible only" tooltip="Return only visible rows/columns. Implied when sorting by Natural.">
            <InlineSwitch
              value={!!visibleOnly}
              onChange={(e) => {
                update({ visibleOnly: e.currentTarget.checked });
                onRunQuery();
              }}
            />
          </InlineField>
        </InlineFieldRow>
      )}

      {!isCount && (
        <div className="gf-form">
          <InlineField
            label="Value format"
            labelWidth={LABEL_WIDTH}
            tooltip="How cell values are returned. Simple is recommended; Rich gives lossless encoding (Markdown / JSON-LD)."
          >
            <RadioButtonGroup<CodaValueFormat>
              options={VALUE_FORMAT_OPTIONS}
              value={valueFormat ?? 'simple'}
              onChange={(v) => {
                update({ valueFormat: v });
                onRunQuery();
              }}
            />
          </InlineField>
        </div>
      )}

      {!isCount && (
        <div className="gf-form">
          <InlineField label="Limit" labelWidth={LABEL_WIDTH} tooltip="Maximum number of rows. 0 returns all (auto-paginated).">
            <Input width={20} type="number" min={0} value={limit ?? 0} onChange={onLimitChange} onBlur={onRunQuery} />
          </InlineField>
        </div>
      )}
    </div>
  );
}
