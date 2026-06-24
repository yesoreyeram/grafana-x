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
} from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';

import { DataSource } from '../datasource';
import { FieldInfo, StrapiDataSourceOptions, StrapiQuery, StrapiQueryType } from '../types';
import { parseSort, serializeSort, SortDirection, SortItem } from '../sort';
import { emptyRootGroup, FilterGroup, parseFilterTree, stringifyFilterTree } from '../filter';
import { FilterEditor } from './FilterEditor';

type Props = QueryEditorProps<DataSource, StrapiQuery, StrapiDataSourceOptions>;

const LABEL_WIDTH = 16;

const QUERY_TYPE_OPTIONS: Array<SelectableValue<StrapiQueryType>> = [
  { label: 'Records', value: 'records', description: 'Return matching records' },
  { label: 'Count', value: 'count', description: 'Return the number of matching records' },
];

const DIRECTION_OPTIONS: Array<SelectableValue<SortDirection>> = [
  { label: 'Asc', value: 'asc' },
  { label: 'Desc', value: 'desc' },
];

function contentTypeChangePatch(contentTypeId: string, changingContentType: boolean): Partial<StrapiQuery> {
  if (!changingContentType) {
    return { contentTypeId };
  }
  return { contentTypeId, filterTree: '', sort: '', fields: '', populate: '' };
}

export function QueryEditor({ query, onChange, onRunQuery, datasource }: Props) {
  const { contentTypeId, sort, fields, filterTree, page, pageSize, populate } = query;
  const queryType = query.queryType ?? 'records';
  const isCount = queryType === 'count';

  const [contentTypes, setContentTypes] = useState<Array<SelectableValue<string>>>([]);
  const [loadingContentTypes, setLoadingContentTypes] = useState(false);
  const [contentTypesError, setContentTypesError] = useState<string | undefined>();

  const [fieldList, setFieldList] = useState<FieldInfo[]>([]);
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

  // Load content types
  useEffect(() => {
    let cancelled = false;
    setLoadingContentTypes(true);
    setContentTypesError(undefined);
    datasource
      .getContentTypes()
      .then((res) => {
        if (!cancelled) {
          setContentTypes(res.map((c) => ({ label: c.displayName || c.pluralName, value: c.pluralName, description: c.uid })));
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setContentTypesError(err?.data?.error ?? err?.message ?? 'Failed to load content types');
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoadingContentTypes(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [datasource]);

  // Load fields for the selected content type
  useEffect(() => {
    let cancelled = false;
    if (!contentTypeId) {
      setFieldList([]);
      setFieldOptions([]);
      setFieldsError(undefined);
      return;
    }
    setLoadingFields(true);
    setFieldsError(undefined);
    datasource
      .getFields(contentTypeId)
      .then((res) => {
        if (!cancelled) {
          setFieldList(res);
          setFieldOptions(res.map((f) => ({ label: f.field, value: f.field, description: f.type })));
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
  }, [contentTypeId, datasource]);

  const update = useCallback(
    (patch: Partial<StrapiQuery>) => {
      onChange({ ...query, ...patch });
    },
    [onChange, query]
  );

  const onQueryTypeChange = (value: StrapiQueryType) => {
    update({ queryType: value });
    onRunQuery();
  };

  const onContentTypeSelect = (value: SelectableValue<string> | null) => {
    const id = value?.value ?? '';
    const changingContentType = id !== contentTypeId;
    if (changingContentType) {
      setFilterRoot(emptyRootGroup());
      setSortItems([]);
    }
    update(contentTypeChangePatch(id, changingContentType));
    onRunQuery();
  };

  const selectedContentType: SelectableValue<string> | null = contentTypeId
    ? (contentTypes.find((c) => c.value === contentTypeId) ?? { label: contentTypeId, value: contentTypeId })
    : null;

  const onPageChange = (e: ChangeEvent<HTMLInputElement>) => {
    const n = parseInt(e.target.value, 10);
    update({ page: isNaN(n) ? 1 : n });
  };

  const onPageSizeChange = (e: ChangeEvent<HTMLInputElement>) => {
    const n = parseInt(e.target.value, 10);
    update({ pageSize: isNaN(n) ? 25 : n });
  };

  const onPopulateChange = (e: ChangeEvent<HTMLInputElement>) => {
    update({ populate: e.target.value });
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
          tooltip="Records returns rows; Count returns the number of matching records (respects filters)."
        >
          <RadioButtonGroup<StrapiQueryType>
            options={QUERY_TYPE_OPTIONS}
            value={queryType}
            onChange={onQueryTypeChange}
          />
        </InlineField>
      </div>

      <div className="gf-form">
        <InlineField
          label="Content Type"
          labelWidth={LABEL_WIDTH}
          tooltip="Type the content type plural API id (e.g. 'articles'). Automatic discovery uses Strapi's content-type-builder endpoint, which needs an admin JWT and is unavailable with an API token — so the list is usually empty and you should type the value directly."
          error={contentTypesError}
          invalid={!!contentTypesError}
          required
        >
          <Select<string>
            width={40}
            isClearable
            isLoading={loadingContentTypes}
            options={contentTypes}
            value={selectedContentType}
            onChange={onContentTypeSelect}
            allowCustomValue
            placeholder="e.g. articles"
            noOptionsMessage="Type a content type plural API id"
          />
        </InlineField>
      </div>

      {!isCount && (
        <div className="gf-form">
          <InlineField
            label="Fields"
            labelWidth={LABEL_WIDTH}
            tooltip="Fields to return. Leave empty to return all fields. Field discovery needs an admin JWT (unavailable with an API token), so type field names directly when the list is empty."
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
              placeholder={contentTypeId ? 'All fields' : 'Select a content type first'}
              disabled={!contentTypeId}
              noOptionsMessage="Type a field name"
            />
          </InlineField>
        </div>
      )}

      {!isCount && (
        <div className="gf-form">
          <InlineField
            label="Populate"
            labelWidth={LABEL_WIDTH}
            tooltip="Relations, components and media to populate. Use * to populate all first-level relations, or a comma-separated list of relation names (e.g. author, tags). Populated relations are returned as JSON."
          >
            <Input
              width={40}
              value={populate ?? ''}
              placeholder="* or author, tags"
              onChange={onPopulateChange}
              onBlur={onRunQuery}
              disabled={!contentTypeId}
            />
          </InlineField>
        </div>
      )}

      <div className="gf-form" style={{ alignItems: 'flex-start' }}>
        <InlineField
          label="Filters"
          labelWidth={LABEL_WIDTH}
          tooltip="Filter records. Add individual filters or nested filter groups. Compiled into Strapi filter params server-side. Operators adapt to each field's type."
        >
          <FilterEditor group={filterRoot} fields={fieldList} disabled={!contentTypeId} onChange={onFilterChange} />
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
                          ? (fieldOptions.find((o) => o.value === item.field) ?? {
                              label: item.field,
                              value: item.field,
                            })
                          : null
                      }
                      onChange={(v) => onSortFieldChange(index, v)}
                      allowCustomValue
                      placeholder="Select field"
                      disabled={!contentTypeId}
                      noOptionsMessage="No fields found"
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
                <Button variant="secondary" size="sm" icon="plus" onClick={onAddSort} disabled={!contentTypeId}>
                  Add sort
                </Button>
              </div>
            </InlineField>
          </div>

          <InlineFieldRow>
            <InlineField
              label="Page"
              labelWidth={LABEL_WIDTH}
              tooltip="Page number (page-based pagination, starts at 1)."
            >
              <Input width={16} type="number" min={1} value={page ?? 1} onChange={onPageChange} onBlur={onRunQuery} />
            </InlineField>

            <InlineField
              label="Page Size"
              tooltip="Number of records per page."
            >
              <Input width={16} type="number" min={1} value={pageSize ?? 25} onChange={onPageSizeChange} onBlur={onRunQuery} />
            </InlineField>
          </InlineFieldRow>
        </>
      )}
    </div>
  );
}
