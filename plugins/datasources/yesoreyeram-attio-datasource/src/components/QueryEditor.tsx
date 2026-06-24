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
import { AttributeInfo, AttioDataSourceOptions, AttioQuery, AttioQueryType } from '../types';
import { parseSort, serializeSort, SortDirection, SortItem } from '../sort';
import { emptyRootGroup, FilterGroup, parseFilterTree, stringifyFilterTree } from '../filter';
import { FilterEditor } from './FilterEditor';

type Props = QueryEditorProps<DataSource, AttioQuery, AttioDataSourceOptions>;

const LABEL_WIDTH = 16;

const QUERY_TYPE_OPTIONS: Array<SelectableValue<AttioQueryType>> = [
  { label: 'Records', value: 'records', description: 'Return matching records' },
  { label: 'Count', value: 'count', description: 'Return the number of matching records' },
];

const DIRECTION_OPTIONS: Array<SelectableValue<SortDirection>> = [
  { label: 'Asc', value: 'asc' },
  { label: 'Desc', value: 'desc' },
];

function objectChangePatch(objectId: string, changingObject: boolean): Partial<AttioQuery> {
  if (!changingObject) {
    return { objectId };
  }
  return { objectId, filterTree: '', sort: '', fields: '' };
}

export function QueryEditor({ query, onChange, onRunQuery, datasource }: Props) {
  const { objectId, sort, fields, filterTree, limit, offset } = query;
  const queryType = query.queryType ?? 'records';
  const isCount = queryType === 'count';

  const [objects, setObjects] = useState<Array<SelectableValue<string>>>([]);
  const [loadingObjects, setLoadingObjects] = useState(false);
  const [objectsError, setObjectsError] = useState<string | undefined>();

  const [fieldList, setFieldList] = useState<AttributeInfo[]>([]);
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

  // Load objects
  useEffect(() => {
    let cancelled = false;
    setLoadingObjects(true);
    setObjectsError(undefined);
    datasource
      .getObjects()
      .then((res) => {
        if (!cancelled) {
          setObjects(
            res.map((o) => ({
              label: o.plural_noun || o.singular_noun || o.api_slug,
              value: o.api_slug,
              description: o.api_slug,
            }))
          );
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setObjectsError(err?.data?.error ?? err?.message ?? 'Failed to load objects');
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoadingObjects(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [datasource]);

  // Load attributes for the selected object
  useEffect(() => {
    let cancelled = false;
    if (!objectId) {
      setFieldList([]);
      setFieldOptions([]);
      setFieldsError(undefined);
      return;
    }
    setLoadingFields(true);
    setFieldsError(undefined);
    datasource
      .getAttributes(objectId)
      .then((res) => {
        if (!cancelled) {
          setFieldList(res);
          setFieldOptions(res.map((f) => ({ label: f.title || f.api_slug, value: f.api_slug, description: f.type })));
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setFieldsError(err?.data?.error ?? err?.message ?? 'Failed to load attributes');
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
  }, [objectId, datasource]);

  const update = useCallback(
    (patch: Partial<AttioQuery>) => {
      onChange({ ...query, ...patch });
    },
    [onChange, query]
  );

  const onQueryTypeChange = (value: AttioQueryType) => {
    update({ queryType: value });
    onRunQuery();
  };

  const onObjectSelect = (value: SelectableValue<string> | null) => {
    const id = value?.value ?? '';
    const changingObject = id !== objectId;
    if (changingObject) {
      setFilterRoot(emptyRootGroup());
      setSortItems([]);
    }
    update(objectChangePatch(id, changingObject));
    onRunQuery();
  };

  const selectedObject: SelectableValue<string> | null = objectId
    ? (objects.find((o) => o.value === objectId) ?? { label: objectId, value: objectId })
    : null;

  const onLimitChange = (e: ChangeEvent<HTMLInputElement>) => {
    const n = parseInt(e.target.value, 10);
    update({ limit: isNaN(n) ? 0 : n });
  };

  const onOffsetChange = (e: ChangeEvent<HTMLInputElement>) => {
    const n = parseInt(e.target.value, 10);
    update({ offset: isNaN(n) ? 0 : n });
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
          <RadioButtonGroup<AttioQueryType>
            options={QUERY_TYPE_OPTIONS}
            value={queryType}
            onChange={onQueryTypeChange}
          />
        </InlineField>
      </div>

      <div className="gf-form">
        <InlineField
          label="Object"
          labelWidth={LABEL_WIDTH}
          tooltip="Select an Attio object (e.g. People, Companies, Deals). The list is fetched from the Attio API."
          error={objectsError}
          invalid={!!objectsError}
          required
        >
          <Select<string>
            width={40}
            isClearable
            isLoading={loadingObjects}
            options={objects}
            value={selectedObject}
            onChange={onObjectSelect}
            allowCustomValue
            placeholder="Select object"
            noOptionsMessage="No objects found"
          />
        </InlineField>
      </div>

      {!isCount && (
        <div className="gf-form">
          <InlineField
            label="Attributes"
            labelWidth={LABEL_WIDTH}
            tooltip="Attributes (columns) to return. Leave empty to return all attributes."
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
              placeholder={objectId ? 'All attributes' : 'Select an object first'}
              disabled={!objectId}
              noOptionsMessage="No attributes found"
            />
          </InlineField>
        </div>
      )}

      <div className="gf-form" style={{ alignItems: 'flex-start' }}>
        <InlineField
          label="Filters"
          labelWidth={LABEL_WIDTH}
          tooltip="Filter records. Add individual filters or nested filter groups. Compiled into an Attio JSON filter object server-side. Operators adapt to each attribute's type."
        >
          <FilterEditor group={filterRoot} fields={fieldList} disabled={!objectId} onChange={onFilterChange} />
        </InlineField>
      </div>

      {!isCount && (
        <>
          <div className="gf-form" style={{ alignItems: 'flex-start' }}>
            <InlineField label="Sort" labelWidth={LABEL_WIDTH} tooltip="Order results by one or more attributes.">
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
                      placeholder="Select attribute"
                      disabled={!objectId}
                      noOptionsMessage="No attributes found"
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
                <Button variant="secondary" size="sm" icon="plus" onClick={onAddSort} disabled={!objectId}>
                  Add sort
                </Button>
              </div>
            </InlineField>
          </div>

          <InlineFieldRow>
            <InlineField
              label="Limit"
              labelWidth={LABEL_WIDTH}
              tooltip="Maximum number of records. 0 returns all (auto-paginated, 500 records/request)."
            >
              <Input width={20} type="number" min={0} value={limit ?? 0} onChange={onLimitChange} onBlur={onRunQuery} />
            </InlineField>

            <InlineField label="Offset" tooltip="Number of records to skip (offset-based pagination).">
              <Input
                width={20}
                type="number"
                min={0}
                value={offset ?? 0}
                onChange={onOffsetChange}
                onBlur={onRunQuery}
              />
            </InlineField>
          </InlineFieldRow>
        </>
      )}
    </div>
  );
}
